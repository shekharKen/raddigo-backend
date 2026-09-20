package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/utils"
)

// SubscriptionRepository defines persistence operations for partner
// subscriptions, their billing transactions, and promo codes.
type SubscriptionRepository interface {
	PartnerExists(ctx context.Context, partnerID string) (bool, error)
	GetActiveByPartner(ctx context.Context, partnerID string) (model.PartnerSubscription, error)
	HasActive(ctx context.Context, partnerID string) (bool, error)
	GetPromoCode(ctx context.Context, code string) (model.PromoCode, error)
	Create(ctx context.Context, sub *model.PartnerSubscription, txn *model.SubscriptionTransaction, promoCodeID string) error
	Upgrade(ctx context.Context, partnerID string, endDate time.Time, txn *model.SubscriptionTransaction, promoCodeID string) (model.PartnerSubscription, error)
	ListByPartner(ctx context.Context, partnerID string, limit, offset int) ([]model.PartnerSubscription, int64, error)
	ListTransactions(ctx context.Context, partnerID string, limit, offset int) ([]model.SubscriptionTransaction, int64, error)
}

// GormSubscriptionRepository is a GORM-backed SubscriptionRepository.
type GormSubscriptionRepository struct {
	db *gorm.DB
}

// NewGormSubscriptionRepository creates a GormSubscriptionRepository.
func NewGormSubscriptionRepository(db *gorm.DB) *GormSubscriptionRepository {
	return &GormSubscriptionRepository{db: db}
}

// PartnerExists reports whether a partner with the given id exists.
func (r *GormSubscriptionRepository) PartnerExists(ctx context.Context, partnerID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Partner{}).
		Where("id = ?", partnerID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count partner: %w", err)
	}
	return count > 0, nil
}

// GetActiveByPartner returns a partner's current active subscription. It
// returns utils.ErrNotFound when the partner has none.
func (r *GormSubscriptionRepository) GetActiveByPartner(ctx context.Context, partnerID string) (model.PartnerSubscription, error) {
	var sub model.PartnerSubscription
	if err := r.db.WithContext(ctx).
		Where("partner_id = ? AND status = ?", partnerID, model.SubscriptionActive).
		Order("created_at DESC").
		First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.PartnerSubscription{}, utils.ErrNotFound
		}
		return model.PartnerSubscription{}, fmt.Errorf("get active subscription: %w", err)
	}
	return sub, nil
}

// HasActive reports whether a partner currently has an active, unexpired
// subscription.
func (r *GormSubscriptionRepository) HasActive(ctx context.Context, partnerID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.PartnerSubscription{}).
		Where("partner_id = ? AND status = ? AND end_date > ?", partnerID, model.SubscriptionActive, time.Now()).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count active subscription: %w", err)
	}
	return count > 0, nil
}

// GetPromoCode looks up a promo code by its (case-sensitive, pre-normalized)
// code. It returns utils.ErrNotFound when no such code exists.
func (r *GormSubscriptionRepository) GetPromoCode(ctx context.Context, code string) (model.PromoCode, error) {
	var promo model.PromoCode
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&promo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.PromoCode{}, utils.ErrNotFound
		}
		return model.PromoCode{}, fmt.Errorf("get promo code: %w", err)
	}
	return promo, nil
}

// Create stores a new partner subscription and its purchase transaction, and
// increments the redeemed count of the applied promo code, if any, all
// within a single transaction.
func (r *GormSubscriptionRepository) Create(ctx context.Context, sub *model.PartnerSubscription, txn *model.SubscriptionTransaction, promoCodeID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(sub).Error; err != nil {
			return fmt.Errorf("create subscription: %w", err)
		}
		txn.SubscriptionID = sub.ID
		if err := tx.Create(txn).Error; err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}
		return redeemPromoCode(tx, promoCodeID)
	})
}

// Upgrade transitions a partner's active monthly subscription to annual in
// place and records the upgrade transaction. It returns utils.ErrNotFound
// when the partner has no active subscription and utils.ErrInvalidState when
// the active subscription is not monthly.
func (r *GormSubscriptionRepository) Upgrade(ctx context.Context, partnerID string, endDate time.Time, txn *model.SubscriptionTransaction, promoCodeID string) (model.PartnerSubscription, error) {
	var updated model.PartnerSubscription
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sub model.PartnerSubscription
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("partner_id = ? AND status = ?", partnerID, model.SubscriptionActive).
			Order("created_at DESC").
			First(&sub).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return utils.ErrNotFound
			}
			return fmt.Errorf("load subscription: %w", err)
		}
		if sub.Plan != model.PlanMonthly {
			return utils.ErrInvalidState
		}

		if err := tx.Model(&model.PartnerSubscription{}).
			Where("id = ?", sub.ID).
			Updates(map[string]interface{}{"plan": model.PlanAnnual, "end_date": endDate}).Error; err != nil {
			return fmt.Errorf("upgrade subscription: %w", err)
		}

		txn.SubscriptionID = sub.ID
		txn.PartnerID = partnerID
		if err := tx.Create(txn).Error; err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		if err := redeemPromoCode(tx, promoCodeID); err != nil {
			return err
		}

		return tx.Where("id = ?", sub.ID).First(&updated).Error
	})
	if err != nil {
		return model.PartnerSubscription{}, err
	}
	return updated, nil
}

// ListByPartner returns a paginated set of a partner's subscriptions, newest
// first, along with the total count.
func (r *GormSubscriptionRepository) ListByPartner(ctx context.Context, partnerID string, limit, offset int) ([]model.PartnerSubscription, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).
		Model(&model.PartnerSubscription{}).
		Where("partner_id = ?", partnerID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count subscriptions: %w", err)
	}

	var subs []model.PartnerSubscription
	if err := r.db.WithContext(ctx).
		Where("partner_id = ?", partnerID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&subs).Error; err != nil {
		return nil, 0, fmt.Errorf("list subscriptions: %w", err)
	}
	return subs, total, nil
}

// ListTransactions returns a paginated set of a partner's subscription
// transactions, newest first, along with the total count.
func (r *GormSubscriptionRepository) ListTransactions(ctx context.Context, partnerID string, limit, offset int) ([]model.SubscriptionTransaction, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).
		Model(&model.SubscriptionTransaction{}).
		Where("partner_id = ?", partnerID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	var txns []model.SubscriptionTransaction
	if err := r.db.WithContext(ctx).
		Where("partner_id = ?", partnerID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&txns).Error; err != nil {
		return nil, 0, fmt.Errorf("list transactions: %w", err)
	}
	return txns, total, nil
}

// redeemPromoCode increments a promo code's redeemed count within tx. A blank
// id is a no-op (no promo code was applied).
func redeemPromoCode(tx *gorm.DB, promoCodeID string) error {
	if promoCodeID == "" {
		return nil
	}
	if err := tx.Model(&model.PromoCode{}).
		Where("id = ?", promoCodeID).
		Update("redeemed", gorm.Expr("redeemed + 1")).Error; err != nil {
		return fmt.Errorf("redeem promo code: %w", err)
	}
	return nil
}
