package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veritasvpn/services/auth-svc/internal/model"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (p *Postgres) CreateAccount(ctx context.Context, acc *model.Account) error {
	query := `INSERT INTO accounts (id, hashed_device_id, hashed_public_key, subscription_tier)
	           VALUES ($1, $2, $3, $4)
	           ON CONFLICT (hashed_device_id) DO UPDATE SET hashed_public_key = $3
	           RETURNING id, created_at, subscription_tier, subscription_expiry, account_status`

	row := p.pool.QueryRow(ctx, query, acc.ID, acc.HashedDeviceID, acc.HashedPublicKey, acc.SubscriptionTier)

	return row.Scan(&acc.ID, &acc.CreatedAt, &acc.SubscriptionTier,
		&acc.SubscriptionExpiry, &acc.AccountStatus)
}

func (p *Postgres) GetAccountByID(ctx context.Context, id string) (*model.Account, error) {
	query := `SELECT id, hashed_device_id, hashed_public_key, created_at,
	           subscription_tier, subscription_expiry, account_status
	           FROM accounts WHERE id = $1 AND account_status != 'deleted'`

	acc := &model.Account{}
	err := p.pool.QueryRow(ctx, query, id).Scan(
		&acc.ID, &acc.HashedDeviceID, &acc.HashedPublicKey, &acc.CreatedAt,
		&acc.SubscriptionTier, &acc.SubscriptionExpiry, &acc.AccountStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	return acc, nil
}

func (p *Postgres) GetAccountByDeviceID(ctx context.Context, hashedDeviceID string) (*model.Account, error) {
	query := `SELECT id, hashed_device_id, hashed_public_key, created_at,
	           subscription_tier, subscription_expiry, account_status
	           FROM accounts WHERE hashed_device_id = $1 AND account_status != 'deleted'`

	acc := &model.Account{}
	err := p.pool.QueryRow(ctx, query, hashedDeviceID).Scan(
		&acc.ID, &acc.HashedDeviceID, &acc.HashedPublicKey, &acc.CreatedAt,
		&acc.SubscriptionTier, &acc.SubscriptionExpiry, &acc.AccountStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("get account by device: %w", err)
	}
	return acc, nil
}

func (p *Postgres) UpdateAccountTier(ctx context.Context, accountID, tier string, expiry *time.Time) error {
	query := `UPDATE accounts SET subscription_tier = $2, subscription_expiry = $3
	           WHERE id = $1`
	_, err := p.pool.Exec(ctx, query, accountID, tier, expiry)
	return err
}

func (p *Postgres) DeleteAccount(ctx context.Context, accountID string) error {
	query := `UPDATE accounts SET account_status = 'deleted' WHERE id = $1`
	_, err := p.pool.Exec(ctx, query, accountID)
	return err
}

func (p *Postgres) StoreRefreshToken(ctx context.Context, token *model.RefreshToken) error {
	query := `INSERT INTO refresh_tokens (account_id, token_hash, expires_at)
	           VALUES ($1, $2, $3)`
	_, err := p.pool.Exec(ctx, query, token.AccountID, token.TokenHash, token.ExpiresAt)
	return err
}

func (p *Postgres) GetRefreshToken(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	query := `SELECT id, account_id, token_hash, expires_at, created_at
	           FROM refresh_tokens WHERE token_hash = $1 AND expires_at > NOW()`

	token := &model.RefreshToken{}
	err := p.pool.QueryRow(ctx, query, tokenHash).Scan(
		&token.ID, &token.AccountID, &token.TokenHash, &token.ExpiresAt, &token.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return token, nil
}

func (p *Postgres) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	query := `DELETE FROM refresh_tokens WHERE token_hash = $1`
	_, err := p.pool.Exec(ctx, query, tokenHash)
	return err
}

func (p *Postgres) DeleteAllRefreshTokens(ctx context.Context, accountID string) error {
	query := `DELETE FROM refresh_tokens WHERE account_id = $1`
	_, err := p.pool.Exec(ctx, query, accountID)
	return err
}
