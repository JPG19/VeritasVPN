package model

import "time"

type Subscription struct {
	ID                  string    `json:"id"`
	AccountID           string    `json:"account_id"`
	Tier                string    `json:"tier"`
	Status              string    `json:"status"`
	PaymentMethod       string    `json:"payment_method"`
	CurrentPeriodStart  time.Time `json:"current_period_start"`
	CurrentPeriodEnd    time.Time `json:"current_period_end"`
	CancelAtPeriodEnd   bool      `json:"cancel_at_period_end"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type PaymentRecord struct {
	ID                    string    `json:"id"`
	SubscriptionID       string    `json:"subscription_id"`
	Amount                int64     `json:"amount"`
	Currency              string    `json:"currency"`
	Status                string    `json:"status"`
	ProviderTransactionID string    `json:"provider_transaction_id"`
	CreatedAt             time.Time `json:"created_at"`
}
