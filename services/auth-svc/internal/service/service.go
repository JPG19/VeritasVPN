package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/veritasvpn/lib/crypto"
	jwtlib "github.com/veritasvpn/lib/jwt"
	"github.com/veritasvpn/lib/logging"
	"github.com/veritasvpn/services/auth-svc/internal/model"
	"github.com/veritasvpn/services/auth-svc/internal/repository"
	"go.uber.org/zap"
)

type AuthService struct {
	log   *logging.Logger
	db    *repository.Postgres
	redis *repository.Redis
	jwt   *jwtlib.Manager
}

func New(log *logging.Logger, db *repository.Postgres, redis *repository.Redis, jwt *jwtlib.Manager) *AuthService {
	return &AuthService{log: log, db: db, redis: redis, jwt: jwt}
}

func hashInput(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}

func (s *AuthService) Register(ctx context.Context, deviceID, publicKey string) (string, string, string, int64, error) {
	if deviceID == "" || publicKey == "" {
		return "", "", "", 0, fmt.Errorf("device_id and public_key are required")
	}

	hashedDeviceID := hashInput(deviceID)
	hashedPublicKey := hashInput(publicKey)

	accountID, err := crypto.GenerateAccountID()
	if err != nil {
		return "", "", "", 0, fmt.Errorf("generate account id: %w", err)
	}

	acc := &model.Account{
		ID:               accountID,
		HashedDeviceID:   hashedDeviceID,
		HashedPublicKey:  hashedPublicKey,
		SubscriptionTier: "free",
	}

	if err := s.db.CreateAccount(ctx, acc); err != nil {
		s.log.Error("failed to create account", zap.Error(err))
		return "", "", "", 0, fmt.Errorf("create account: %w", err)
	}

	accessToken, expiresAt, err := s.jwt.GenerateAccessToken(acc.ID, acc.SubscriptionTier)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := crypto.GenerateRefreshToken()
	if err != nil {
		return "", "", "", 0, fmt.Errorf("generate refresh token: %w", err)
	}

	refreshTokenHash := hashInput(refreshToken)
	rt := &model.RefreshToken{
		AccountID: acc.ID,
		TokenHash: refreshTokenHash,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}

	if err := s.db.StoreRefreshToken(ctx, rt); err != nil {
		return "", "", "", 0, fmt.Errorf("store refresh token: %w", err)
	}

	s.log.Info("account registered",
		zap.String("account_id", acc.ID),
		zap.String("tier", acc.SubscriptionTier),
	)

	return accessToken, refreshToken, acc.ID, expiresAt, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (string, string, int64, error) {
	tokenHash := hashInput(refreshToken)

	rt, err := s.db.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid refresh token: %w", err)
	}

	if err := s.db.DeleteRefreshToken(ctx, tokenHash); err != nil {
		s.log.Warn("failed to delete old refresh token", zap.Error(err))
	}

	acc, err := s.db.GetAccountByID(ctx, rt.AccountID)
	if err != nil {
		return "", "", 0, fmt.Errorf("get account: %w", err)
	}

	accessToken, expiresAt, err := s.jwt.GenerateAccessToken(acc.ID, acc.SubscriptionTier)
	if err != nil {
		return "", "", 0, fmt.Errorf("generate access token: %w", err)
	}

	newRefreshToken, err := crypto.GenerateRefreshToken()
	if err != nil {
		return "", "", 0, fmt.Errorf("generate refresh token: %w", err)
	}

	newTokenHash := hashInput(newRefreshToken)
	newRT := &model.RefreshToken{
		AccountID: acc.ID,
		TokenHash: newTokenHash,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}

	if err := s.db.StoreRefreshToken(ctx, newRT); err != nil {
		return "", "", 0, fmt.Errorf("store new refresh token: %w", err)
	}

	return accessToken, newRefreshToken, expiresAt, nil
}

func (s *AuthService) ValidateToken(ctx context.Context, accessToken string) (*jwtlib.Claims, error) {
	tokenHash := hashInput(accessToken)

	blacklisted, err := s.redis.IsTokenBlacklisted(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("check blacklist: %w", err)
	}
	if blacklisted {
		return nil, fmt.Errorf("token has been revoked")
	}

	claims, err := s.jwt.ValidateAccessToken(accessToken)
	if err != nil {
		return nil, fmt.Errorf("validate token: %w", err)
	}

	return claims, nil
}

func (s *AuthService) GetAccount(ctx context.Context, accountID string) (*model.Account, error) {
	return s.db.GetAccountByID(ctx, accountID)
}

func (s *AuthService) DeleteAccount(ctx context.Context, accountID string) error {
	if err := s.db.DeleteAccount(ctx, accountID); err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	if err := s.db.DeleteAllRefreshTokens(ctx, accountID); err != nil {
		s.log.Warn("failed to delete refresh tokens", zap.String("account_id", accountID), zap.Error(err))
	}

	s.log.Info("account deleted", zap.String("account_id", accountID))
	return nil
}
