package dto

import "time"

// CreateSubscriptionRequest is the payload accepted when a partner subscribes
// to a plan for the first time.
type CreateSubscriptionRequest struct {
	Plan       string `json:"plan"`
	PromoCode  string `json:"promo_code"`
	PaymentRef string `json:"payment_ref"`
}

// UpgradeSubscriptionRequest is the payload accepted when a partner upgrades
// an active monthly subscription to annual.
type UpgradeSubscriptionRequest struct {
	PromoCode  string `json:"promo_code"`
	PaymentRef string `json:"payment_ref"`
}

// SubscriptionResponse is the API representation of a partner subscription.
type SubscriptionResponse struct {
	ID        string    `json:"id"`
	PartnerID string    `json:"partner_id"`
	Plan      string    `json:"plan"`
	Status    string    `json:"status"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	AutoRenew bool      `json:"auto_renew"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TransactionResponse is the API representation of a subscription transaction.
type TransactionResponse struct {
	ID             string    `json:"id"`
	SubscriptionID string    `json:"subscription_id"`
	PartnerID      string    `json:"partner_id"`
	Type           string    `json:"type"`
	Plan           string    `json:"plan"`
	Amount         float64   `json:"amount"`
	DiscountAmount float64   `json:"discount_amount"`
	PromoCode      string    `json:"promo_code,omitempty"`
	Status         string    `json:"status"`
	PaymentRef     string    `json:"payment_ref,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// SubscriptionWithTransaction bundles a subscription with the transaction
// just created for it (purchase or upgrade).
type SubscriptionWithTransaction struct {
	Subscription SubscriptionResponse `json:"subscription"`
	Transaction  TransactionResponse  `json:"transaction"`
}
