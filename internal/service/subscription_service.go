package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/repository"
	"github.com/raddigo/raddigo/internal/utils"
	"github.com/raddigo/raddigo/internal/validation"
)

// SubscriptionService manages partner subscription plans, upgrades, promo
// codes and their billing transactions.
type SubscriptionService struct {
	repo         repository.SubscriptionRepository
	monthlyPrice float64
	annualPrice  float64
	now          func() time.Time
	id           func() string
}

// NewSubscriptionService creates a SubscriptionService. monthlyPrice and
// annualPrice are the base plan prices before any promo code discount.
func NewSubscriptionService(repo repository.SubscriptionRepository, monthlyPrice, annualPrice float64) *SubscriptionService {
	return &SubscriptionService{
		repo:         repo,
		monthlyPrice: monthlyPrice,
		annualPrice:  annualPrice,
		now:          time.Now,
		id:           func() string { return uuid.NewString() },
	}
}

// Subscribe validates and stores a partner's first subscription to a plan,
// applying a promo code discount when provided. It returns
// utils.ErrInvalidState when the partner already has an active subscription.
func (s *SubscriptionService) Subscribe(ctx context.Context, partnerID string, in dto.CreateSubscriptionRequest) (dto.SubscriptionWithTransaction, error) {
	if err := validation.ValidateCreateSubscription(in); err != nil {
		return dto.SubscriptionWithTransaction{}, err
	}
	if err := s.ensurePartner(ctx, partnerID); err != nil {
		return dto.SubscriptionWithTransaction{}, err
	}

	if _, err := s.repo.GetActiveByPartner(ctx, partnerID); err == nil {
		return dto.SubscriptionWithTransaction{}, utils.ErrInvalidState
	} else if !errors.Is(err, utils.ErrNotFound) {
		return dto.SubscriptionWithTransaction{}, err
	}

	plan := model.SubscriptionPlan(strings.TrimSpace(in.Plan))
	base := s.priceFor(plan)

	amount, discount, promoCodeID, promoCode, err := s.applyPromo(ctx, plan, base, in.PromoCode)
	if err != nil {
		return dto.SubscriptionWithTransaction{}, err
	}

	now := s.now()
	sub := &model.PartnerSubscription{
		ID:        s.id(),
		PartnerID: partnerID,
		Plan:      plan,
		Status:    model.SubscriptionActive,
		StartDate: now,
		EndDate:   endDateFor(now, plan),
		CreatedAt: now,
		UpdatedAt: now,
	}
	txn := &model.SubscriptionTransaction{
		ID:             s.id(),
		PartnerID:      partnerID,
		Type:           model.TransactionPurchase,
		Plan:           plan,
		Amount:         amount,
		DiscountAmount: discount,
		PromoCode:      promoCode,
		Status:         model.TransactionSuccess,
		PaymentRef:     strings.TrimSpace(in.PaymentRef),
		CreatedAt:      now,
	}
	if err := s.repo.Create(ctx, sub, txn, promoCodeID); err != nil {
		return dto.SubscriptionWithTransaction{}, err
	}
	return dto.SubscriptionWithTransaction{
		Subscription: toSubscriptionResponse(*sub),
		Transaction:  toTransactionResponse(*txn),
	}, nil
}

// Upgrade upgrades a partner's active monthly subscription to annual,
// charging the difference between the two plan prices minus any promo
// discount. It returns utils.ErrNotFound when the partner has no active
// subscription and utils.ErrInvalidState when it is already annual.
func (s *SubscriptionService) Upgrade(ctx context.Context, partnerID string, in dto.UpgradeSubscriptionRequest) (dto.SubscriptionWithTransaction, error) {
	if err := validation.ValidateUpgradeSubscription(in); err != nil {
		return dto.SubscriptionWithTransaction{}, err
	}
	if err := s.ensurePartner(ctx, partnerID); err != nil {
		return dto.SubscriptionWithTransaction{}, err
	}

	current, err := s.repo.GetActiveByPartner(ctx, partnerID)
	if err != nil {
		return dto.SubscriptionWithTransaction{}, err
	}
	if current.Plan != model.PlanMonthly {
		return dto.SubscriptionWithTransaction{}, utils.ErrInvalidState
	}

	diff := s.annualPrice - s.monthlyPrice
	if diff < 0 {
		diff = 0
	}
	amount, discount, promoCodeID, promoCode, err := s.applyPromo(ctx, model.PlanAnnual, diff, in.PromoCode)
	if err != nil {
		return dto.SubscriptionWithTransaction{}, err
	}

	now := s.now()
	txn := &model.SubscriptionTransaction{
		ID:             s.id(),
		Type:           model.TransactionUpgrade,
		Plan:           model.PlanAnnual,
		Amount:         amount,
		DiscountAmount: discount,
		PromoCode:      promoCode,
		Status:         model.TransactionSuccess,
		PaymentRef:     strings.TrimSpace(in.PaymentRef),
		CreatedAt:      now,
	}
	updated, err := s.repo.Upgrade(ctx, partnerID, endDateFor(now, model.PlanAnnual), txn, promoCodeID)
	if err != nil {
		return dto.SubscriptionWithTransaction{}, err
	}
	return dto.SubscriptionWithTransaction{
		Subscription: toSubscriptionResponse(updated),
		Transaction:  toTransactionResponse(*txn),
	}, nil
}

// GetActive returns a partner's current active subscription.
func (s *SubscriptionService) GetActive(ctx context.Context, partnerID string) (dto.SubscriptionResponse, error) {
	sub, err := s.repo.GetActiveByPartner(ctx, partnerID)
	if err != nil {
		return dto.SubscriptionResponse{}, err
	}
	return toSubscriptionResponse(sub), nil
}

// IsSubscribed reports whether a partner currently has an active, unexpired
// subscription. Used to enrich login/register responses.
func (s *SubscriptionService) IsSubscribed(ctx context.Context, partnerID string) (bool, error) {
	return s.repo.HasActive(ctx, partnerID)
}

// ListForPartner returns a partner's subscription history, newest first.
func (s *SubscriptionService) ListForPartner(ctx context.Context, partnerID string, page, pageSize int) (dto.PageResult[dto.SubscriptionResponse], error) {
	page, pageSize = dto.NormalizePageParams(page, pageSize)
	offset := (page - 1) * pageSize

	subs, total, err := s.repo.ListByPartner(ctx, partnerID, pageSize, offset)
	if err != nil {
		return dto.PageResult[dto.SubscriptionResponse]{}, err
	}
	out := make([]dto.SubscriptionResponse, 0, len(subs))
	for _, sub := range subs {
		out = append(out, toSubscriptionResponse(sub))
	}
	return dto.PageResult[dto.SubscriptionResponse]{
		Data:       out,
		Pagination: dto.NewPagination(page, pageSize, total),
	}, nil
}

// ListTransactions returns a partner's subscription billing history, newest first.
func (s *SubscriptionService) ListTransactions(ctx context.Context, partnerID string, page, pageSize int) (dto.PageResult[dto.TransactionResponse], error) {
	page, pageSize = dto.NormalizePageParams(page, pageSize)
	offset := (page - 1) * pageSize

	txns, total, err := s.repo.ListTransactions(ctx, partnerID, pageSize, offset)
	if err != nil {
		return dto.PageResult[dto.TransactionResponse]{}, err
	}
	out := make([]dto.TransactionResponse, 0, len(txns))
	for _, t := range txns {
		out = append(out, toTransactionResponse(t))
	}
	return dto.PageResult[dto.TransactionResponse]{
		Data:       out,
		Pagination: dto.NewPagination(page, pageSize, total),
	}, nil
}

// applyPromo validates an optional promo code against the plan and computes
// the final amount and discount off base. An empty code returns the base
// amount unchanged.
func (s *SubscriptionService) applyPromo(ctx context.Context, plan model.SubscriptionPlan, base float64, code string) (amount, discount float64, promoCodeID, appliedCode string, err error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return base, 0, "", "", nil
	}

	promo, err := s.repo.GetPromoCode(ctx, code)
	if err != nil {
		if errors.Is(err, utils.ErrNotFound) {
			return 0, 0, "", "", utils.NewValidationError("promo code is invalid")
		}
		return 0, 0, "", "", err
	}

	now := s.now()
	if !promo.Active || now.Before(promo.ValidFrom) || now.After(promo.ValidUntil) {
		return 0, 0, "", "", utils.NewValidationError("promo code has expired or is not yet active")
	}
	if promo.Plan != "" && promo.Plan != plan {
		return 0, 0, "", "", utils.NewValidationError("promo code is not applicable to this plan")
	}
	if promo.MaxRedemptions > 0 && promo.Redeemed >= promo.MaxRedemptions {
		return 0, 0, "", "", utils.NewValidationError("promo code has reached its redemption limit")
	}

	discount = base * promo.DiscountPercent / 100
	if discount > base {
		discount = base
	}
	return base - discount, discount, promo.ID, promo.Code, nil
}

func (s *SubscriptionService) priceFor(plan model.SubscriptionPlan) float64 {
	if plan == model.PlanAnnual {
		return s.annualPrice
	}
	return s.monthlyPrice
}

func (s *SubscriptionService) ensurePartner(ctx context.Context, partnerID string) error {
	exists, err := s.repo.PartnerExists(ctx, partnerID)
	if err != nil {
		return err
	}
	if !exists {
		return utils.ErrNotFound
	}
	return nil
}

// endDateFor computes a plan's expiry from its start date.
func endDateFor(start time.Time, plan model.SubscriptionPlan) time.Time {
	if plan == model.PlanAnnual {
		return start.AddDate(1, 0, 0)
	}
	return start.AddDate(0, 1, 0)
}

func toSubscriptionResponse(sub model.PartnerSubscription) dto.SubscriptionResponse {
	return dto.SubscriptionResponse{
		ID:        sub.ID,
		PartnerID: sub.PartnerID,
		Plan:      string(sub.Plan),
		Status:    string(sub.Status),
		StartDate: sub.StartDate,
		EndDate:   sub.EndDate,
		AutoRenew: sub.AutoRenew,
		CreatedAt: sub.CreatedAt,
		UpdatedAt: sub.UpdatedAt,
	}
}

func toTransactionResponse(t model.SubscriptionTransaction) dto.TransactionResponse {
	return dto.TransactionResponse{
		ID:             t.ID,
		SubscriptionID: t.SubscriptionID,
		PartnerID:      t.PartnerID,
		Type:           string(t.Type),
		Plan:           string(t.Plan),
		Amount:         t.Amount,
		DiscountAmount: t.DiscountAmount,
		PromoCode:      t.PromoCode,
		Status:         string(t.Status),
		PaymentRef:     t.PaymentRef,
		CreatedAt:      t.CreatedAt,
	}
}
