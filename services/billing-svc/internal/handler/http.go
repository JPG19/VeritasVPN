package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/veritasvpn/lib/logging"
	"github.com/veritasvpn/services/billing-svc/internal/service"
	"go.uber.org/zap"
)

type BillingHandler struct {
	log     *logging.Logger
	service *service.BillingService
}

func NewBillingHandler(log *logging.Logger, svc *service.BillingService) *BillingHandler {
	return &BillingHandler{log: log, service: svc}
}

func (h *BillingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/billing/subscribe", h.handleSubscribe)
	mux.HandleFunc("/api/v1/billing/cancel", h.handleCancel)
	mux.HandleFunc("/api/v1/billing/status", h.handleStatus)
	mux.HandleFunc("/api/v1/billing/webhook/stripe", h.handleStripeWebhook)
	mux.HandleFunc("/api/v1/billing/webhook/btcpay", h.handleBTCPayWebhook)
}

func (h *BillingHandler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		AccountID     string `json:"account_id"`
		Tier          string `json:"tier"`
		PaymentMethod string `json:"payment_method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	checkoutURL, err := h.service.CreateSubscription(r.Context(), req.AccountID, req.Tier, req.PaymentMethod)
	if err != nil {
		h.log.Error("failed to create subscription", zap.Error(err))
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"checkout_url": checkoutURL,
	})
}

func (h *BillingHandler) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.CancelSubscription(r.Context(), req.AccountID); err != nil {
		h.log.Error("failed to cancel subscription", zap.Error(err))
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *BillingHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	sub, err := h.service.GetSubscription(r.Context(), accountID)
	if err != nil {
		h.log.Error("failed to get subscription", zap.Error(err))
		writeError(w, http.StatusNotFound, "subscription not found")
		return
	}

	writeJSON(w, http.StatusOK, sub)
}

func (h *BillingHandler) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	defer r.Body.Close()

	signature := r.Header.Get("Stripe-Signature")

	if err := h.service.ProcessStripeWebhook(payload, signature); err != nil {
		h.log.Error("stripe webhook failed", zap.Error(err))
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *BillingHandler) handleBTCPayWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	defer r.Body.Close()

	signature := r.Header.Get("BTCPay-Sig")

	if err := h.service.ProcessBTCPayWebhook(payload, signature); err != nil {
		h.log.Error("btcpay webhook failed", zap.Error(err))
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
