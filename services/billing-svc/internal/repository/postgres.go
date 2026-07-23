package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veritasvpn/services/billing-svc/internal/model"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (p *Postgres) CreateSubscription(ctx context.Context, sub *model.Subscription) error {
	query := `INSERT INTO subscriptions (id, account_id, tier, status, payment_method,
	           current_period_start, current_period_end, cancel_at_period_end, created_at, updated_at)
	           VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	           ON CONFLICT (account_id) DO UPDATE SET
	               tier = $3, status = $4, payment_method = $5,
	               current_period_start = $6, current_period_end = $7,
	               cancel_at_period_end = $8, updated_at = $10
	           RETURNING id, created_at, updated_at`

	now := time.Now().UTC()
	sub.CreatedAt = now
	sub.UpdatedAt = now

	row := p.pool.QueryRow(ctx, query,
		sub.ID, sub.AccountID, sub.Tier, sub.Status, sub.PaymentMethod,
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CancelAtPeriodEnd,
		sub.CreatedAt, sub.UpdatedAt,
	)

	return row.Scan(&sub.ID, &sub.CreatedAt, &sub.UpdatedAt)
}

func (p *Postgres) GetSubscription(ctx context.Context, accountID string) (*model.Subscription, error) {
	query := `SELECT id, account_id, tier, status, payment_method,
	           current_period_start, current_period_end, cancel_at_period_end,
	           created_at, updated_at
	           FROM subscriptions WHERE account_id = $1`

	sub := &model.Subscription{}
	err := p.pool.QueryRow(ctx, query, accountID).Scan(
		&sub.ID, &sub.AccountID, &sub.Tier, &sub.Status, &sub.PaymentMethod,
		&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CancelAtPeriodEnd,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	return sub, nil
}

func (p *Postgres) GetSubscriptionByID(ctx context.Context, id string) (*model.Subscription, error) {
	query := `SELECT id, account_id, tier, status, payment_method,
	           current_period_start, current_period_end, cancel_at_period_end,
	           created_at, updated_at
	           FROM subscriptions WHERE id = $1`

	sub := &model.Subscription{}
	err := p.pool.QueryRow(ctx, query, id).Scan(
		&sub.ID, &sub.AccountID, &sub.Tier, &sub.Status, &sub.PaymentMethod,
		&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CancelAtPeriodEnd,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get subscription by id: %w", err)
	}
	return sub, nil
}

func (p *Postgres) UpdateSubscription(ctx context.Context, sub *model.Subscription) error {
	query := `UPDATE subscriptions SET
	           tier = $2, status = $3, payment_method = $4,
	           current_period_start = $5, current_period_end = $6,
	           cancel_at_period_end = $7, updated_at = NOW()
	           WHERE id = $1`

	_, err := p.pool.Exec(ctx, query,
		sub.ID, sub.Tier, sub.Status, sub.PaymentMethod,
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CancelAtPeriodEnd,
	)
	return err
}

func (p *Postgres) CancelSubscription(ctx context.Context, accountID string) error {
	query := `UPDATE subscriptions SET cancel_at_period_end = TRUE, updated_at = NOW()
	           WHERE account_id = $1`

	_, err := p.pool.Exec(ctx, query, accountID)
	return err
}

func (p *Postgres) CreatePaymentRecord(ctx context.Context, pr *model.PaymentRecord) error {
	query := `INSERT INTO payment_records (id, subscription_id, amount, currency,
	           status, provider_transaction_id, created_at)
	           VALUES ($1, $2, $3, $4, $5, $6, $7)
	           RETURNING id, created_at`

	pr.CreatedAt = time.Now().UTC()

	row := p.pool.QueryRow(ctx, query,
		pr.ID, pr.SubscriptionID, pr.Amount, pr.Currency,
		pr.Status, pr.ProviderTransactionID, pr.CreatedAt,
	)

	return row.Scan(&pr.ID, &pr.CreatedAt)
}
