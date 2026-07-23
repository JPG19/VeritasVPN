package provider

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/veritasvpn/lib/logging"
	"go.uber.org/zap"
)

type BTCPayProvider struct {
	log        *logging.Logger
	serverURL  string
	apiKey     string
	httpClient *http.Client
}

func NewBTCPayProvider(log *logging.Logger, serverURL, apiKey string) *BTCPayProvider {
	return &BTCPayProvider{
		log:        log,
		serverURL:  serverURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type BTCPayInvoiceRequest struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Metadata struct {
		AccountID string `json:"account_id"`
		Tier      string `json:"tier"`
	} `json:"metadata"`
	Checkout struct {
		RedirectURL string `json:"redirectURL"`
	} `json:"checkout"`
}

type BTCPayInvoiceResponse struct {
	ID         string `json:"id"`
	CheckoutLink string `json:"checkoutLink"`
	Status     string `json:"status"`
}

func (b *BTCPayProvider) CreateInvoice(accountID, tier string, amount float64) (string, string, error) {
	invReq := BTCPayInvoiceRequest{
		Amount:   amount,
		Currency: "USD",
	}
	invReq.Metadata.AccountID = accountID
	invReq.Metadata.Tier = tier
	invReq.Checkout.RedirectURL = "https://veritasvpn.com/success"

	body, err := json.Marshal(invReq)
	if err != nil {
		return "", "", fmt.Errorf("marshal invoice request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/stores/default/invoices", b.serverURL)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("create btcpay request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+b.apiKey)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("btcpay api call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read btcpay response: %w", err)
	}

	if resp.StatusCode >= 400 {
		b.log.Error("btcpay api error",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)),
		)
		return "", "", fmt.Errorf("btcpay api error: %d %s", resp.StatusCode, string(respBody))
	}

	var invoice BTCPayInvoiceResponse
	if err := json.Unmarshal(respBody, &invoice); err != nil {
		return "", "", fmt.Errorf("unmarshal btcpay response: %w", err)
	}

	return invoice.ID, invoice.CheckoutLink, nil
}

func (b *BTCPayProvider) HandleWebhook(payload []byte, signature string) error {
	if !b.verifySignature(payload, signature) {
		return fmt.Errorf("invalid btcpay webhook signature")
	}

	var event struct {
		Type          string `json:"type"`
		InvoiceID     string `json:"invoiceId"`
		DeliveryID    string `json:"deliveryId"`
		WebhookID     string `json:"webhookId"`
		OriginalDeliveryID string `json:"originalDeliveryId"`
		Timestamp     int64  `json:"timestamp"`
		Metadata      struct {
			AccountID string `json:"account_id"`
			Tier      string `json:"tier"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("unmarshal btcpay event: %w", err)
	}

	b.log.Info("btcpay webhook received",
		zap.String("type", event.Type),
		zap.String("invoice_id", event.InvoiceID),
	)

	switch event.Type {
	case "InvoiceReceivedPayment":
		b.log.Info("btcpay invoice payment received",
			zap.String("invoice_id", event.InvoiceID),
			zap.String("account_id", event.Metadata.AccountID),
			zap.String("tier", event.Metadata.Tier),
		)
		return nil
	case "InvoiceSettled":
		b.log.Info("btcpay invoice settled",
			zap.String("invoice_id", event.InvoiceID),
			zap.String("account_id", event.Metadata.AccountID),
		)
		return nil
	case "InvoiceExpired":
		b.log.Warn("btcpay invoice expired",
			zap.String("invoice_id", event.InvoiceID),
			zap.String("account_id", event.Metadata.AccountID),
		)
		return nil
	default:
		b.log.Debug("unhandled btcpay event type", zap.String("type", event.Type))
		return nil
	}
}

func (b *BTCPayProvider) verifySignature(payload []byte, signature string) bool {
	if b.apiKey == "" {
		return true
	}

	mac := hmac.New(sha256.New, []byte(b.apiKey))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expectedSig), []byte(signature))
}
