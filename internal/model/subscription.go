package model

import "time"

// SubscriptionPlan identifies the billing cycle of a partner subscription.
type SubscriptionPlan string

const (
	PlanMonthly SubscriptionPlan = "monthly"
	PlanAnnual  SubscriptionPlan = "annual"
)

// SubscriptionStatus is the lifecycle state of a partner subscription.
type SubscriptionStatus string

const (
	SubscriptionActive    SubscriptionStatus = "active"
	SubscriptionExpired   SubscriptionStatus = "expired"
	SubscriptionCancelled SubscriptionStatus = "cancelled"
)

// TransactionType identifies what triggered a SubscriptionTransaction.
type TransactionType string

const (
	TransactionPurchase TransactionType = "purchase"
	TransactionUpgrade  TransactionType = "upgrade"
	TransactionRenewal  TransactionType = "renewal"
)

// TransactionStatus is the outcome of a subscription payment.
type TransactionStatus string

const (
	TransactionSuccess TransactionStatus = "success"
	TransactionFailed  TransactionStatus = "failed"
)

// PartnerSubscription is a partner's subscription to a monthly or annual
// plan. A partner has at most one active subscription at a time; upgrading a
// monthly subscription to annual updates this same row in place.
type PartnerSubscription struct {
	ID        string             `json:"id" gorm:"type:uuid;primaryKey"`
	PartnerID string             `json:"partner_id" gorm:"type:uuid;not null;index"`
	Plan      SubscriptionPlan   `json:"plan" gorm:"type:varchar(20);not null"`
	Status    SubscriptionStatus `json:"status" gorm:"type:varchar(20);not null;default:active;index"`
	StartDate time.Time          `json:"start_date"`
	EndDate   time.Time          `json:"end_date"`
	AutoRenew bool               `json:"auto_renew" gorm:"not null;default:false"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// SubscriptionTransaction records a single payment event (purchase, upgrade,
// or renewal) against a partner's subscription, forming a billing history.
type SubscriptionTransaction struct {
	ID             string            `json:"id" gorm:"type:uuid;primaryKey"`
	SubscriptionID string            `json:"subscription_id" gorm:"type:uuid;not null;index"`
	PartnerID      string            `json:"partner_id" gorm:"type:uuid;not null;index"`
	Type           TransactionType   `json:"type" gorm:"type:varchar(20);not null"`
	Plan           SubscriptionPlan  `json:"plan" gorm:"type:varchar(20);not null"`
	Amount         float64           `json:"amount" gorm:"not null"`
	DiscountAmount float64           `json:"discount_amount" gorm:"not null;default:0"`
	PromoCode      string            `json:"promo_code,omitempty"`
	Status         TransactionStatus `json:"status" gorm:"type:varchar(20);not null"`
	PaymentRef     string            `json:"payment_ref,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

// PromoCode is a discount code partners can redeem when subscribing or
// upgrading. Plan, when set, restricts the code to that plan only.
type PromoCode struct {
	ID              string           `json:"id" gorm:"type:uuid;primaryKey"`
	Code            string           `json:"code" gorm:"uniqueIndex;not null"`
	DiscountPercent float64          `json:"discount_percent" gorm:"not null;default:0"`
	Plan            SubscriptionPlan `json:"plan,omitempty" gorm:"type:varchar(20)"`
	MaxRedemptions  int              `json:"max_redemptions" gorm:"not null;default:0"`
	Redeemed        int              `json:"redeemed" gorm:"not null;default:0"`
	ValidFrom       time.Time        `json:"valid_from"`
	ValidUntil      time.Time        `json:"valid_until"`
	Active          bool             `json:"active" gorm:"not null;default:true"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}
