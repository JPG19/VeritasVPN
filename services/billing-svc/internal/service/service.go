package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/veritasvpn/lib/logging"
	"github.com/veritasvpn/services/billing-svc/internal/model"
	"github.com/veritasvpn/services/billing-svc/internal/provider"
	"github.com/veritasvpn/services/billing-svc/internal/repository"
	"go.uber.org/zap"
)

type BillingService struct {
	log            *logging.Logger
	db             *repository.Postgres
	natsConn       *nats.Conn
	stripeProvider *provider.StripeProvider
	btcpayProvider *provider.BTCPayProvider
}

func New(
	log *logging.Logger,
	db *repository.Postgres,
	natsConn *nats.Conn,
	stripe *provider.StripeProvider,
	btcpay *provider.BTCPayProvider,
) *BillingService {
	return &BillingService{
		log:            log,
		db:             db,
		natsConn:       natsConn,
		stripeProvider: stripe,
		btcpayProvider: btcpay,
	}
}

func (s *BillingService) CreateSubscription(ctx context.Context, accountID, tier, paymentMethod string) (string, error) {
	if accountID == "" || tier == "" || paymentMethod == "" {
		return "", fmt.Errorf("account_id, tier, and payment_method are required")
	}

	now := time.Now().UTC()
	sub := &model.Subscription{
		AccountID:          accountID,
		Tier:               tier,
		Status:             "active",
		PaymentMethod:      paymentMethod,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.Add(30 * 24 * time.Hour),
		CancelAtPeriodEnd:  false,
	}

	if err := s.db.CreateSubscription(ctx, sub); err != nil {
		s.log.Error("failed to create subscription", zap.Error(err))
		return "", fmt.Errorf("create subscription: %w", err)
	}

	var checkoutURL string

	switch paymentMethod {
	case "stripe":
		url, sessionID, err := s.stripeProvider.CreateCheckoutSession(accountID, tier)
		if err != nil {
			return "", fmt.Errorf("stripe checkout: %w", err)
		}
		checkoutURL = url

		_ = s.db.CreatePaymentRecord(ctx, &model.PaymentRecord{
			SubscriptionID:       sub.ID,
			Amount:               getAmountForTier(tier),
			Currency:             "usd",
			Status:               "pending",
			ProviderTransactionID: sessionID,
		})

	case "btcpay":
		amount := float64(getAmountForTier(tier)) / 100.0
		invoiceID, url, err := s.btcpayProvider.CreateInvoice(accountID, tier, amount)
		if err != nil {
			return "", fmt.Errorf("btcpay invoice: %w", err)
		}
		checkoutURL = url

		_ = s.db.CreatePaymentRecord(ctx, &model.PaymentRecord{
			SubscriptionID:       sub.ID,
			Amount:               getAmountForTier(tier),
			Currency:             "usd",
			Status:               "pending",
			ProviderTransactionID: invoiceID,
		})

	default:
		return "", fmt.Errorf("unsupported payment method: %s", paymentMethod)
	}

	s.publishEvent("subscription.created", map[string]interface{}{
		"account_id": accountID,
		"tier":       tier,
	})

	s.log.Info("subscription created",
		zap.String("account_id", accountID),
		zap.String("tier", tier),
		zap.String("payment_method", paymentMethod),
	)

	return checkoutURL, nil
}

func (s *BillingService) CancelSubscription(ctx context.Context, accountID string) error {
	if accountID == "" {
		return fmt.Errorf("account_id is required")
	}

	if err := s.db.CancelSubscription(ctx, accountID); err != nil {
		s.log.Error("failed to cancel subscription", zap.Error(err))
		return fmt.Errorf("cancel subscription: %w", err)
	}

	sub, err := s.db.GetSubscription(ctx, accountID)
	if err != nil {
		s.log.Error("failed to get subscription for cancel event", zap.Error(err))
		return fmt.Errorf("get subscription: %w", err)
	}

	s.publishEvent("subscription.canceled", map[string]interface{}{
		"account_id": accountID,
		"tier":       sub.Tier,
	})

	s.log.Info("subscription marked for cancellation",
		zap.String("account_id", accountID),
	)

	return nil
}

func (s *BillingService) GetSubscription(ctx context.Context, accountID string) (*model.Subscription, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account_id is required")
	}

	sub, err := s.db.GetSubscription(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}

	return sub, nil
}

func (s *BillingService) ProcessStripeWebhook(payload []byte, signature string) error {
	if err := s.stripeProvider.HandleWebhook(payload, signature); err != nil {
		s.log.Error("stripe webhook processing failed", zap.Error(err))
		return fmt.Errorf("stripe webhook: %w", err)
	}

	s.log.Info("stripe webhook processed successfully")
	return nil
}

func (s *BillingService) ProcessBTCPayWebhook(payload []byte, signature string) error {
	if err := s.btcpayProvider.HandleWebhook(payload, signature); err != nil {
		s.log.Error("btcpay webhook processing failed", zap.Error(err))
		return fmt.Errorf("btcpay webhook: %w", err)
	}

	s.log.Info("btcpay webhook processed successfully")
	return nil
}

func (s *BillingService) UpdateSubscriptionStatus(ctx context.Context, accountID, status string) error {
	sub, err := s.db.GetSubscription(ctx, accountID)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	sub.Status = status

	if err := s.db.UpdateSubscription(ctx, sub); err != nil {
		return fmt.Errorf("update subscription: %w", err)
	}

	s.publishEvent("subscription.updated", map[string]interface{}{
		"account_id": accountID,
		"tier":       sub.Tier,
		"status":     status,
	})

	return nil
}

func (s *BillingService) publishEvent(subject string, payload map[string]interface{}) {
	if s.natsConn == nil {
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		s.log.Error("failed to marshal event payload", zap.Error(err))
		return
	}

	if err := s.natsConn.Publish(subject, data); err != nil {
		s.log.Error("failed to publish NATS event",
			zap.String("subject", subject),
			zap.Error(err),
		)
	}
}

func getAmountForTier(tier string) int64 {
	switch tier {
	case "premium":
		return 999
	default:
		return 0
	}
}
