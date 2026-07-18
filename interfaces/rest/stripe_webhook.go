package rest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/stripe/stripe-go/v74/webhook"

	domain "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
)

type StripeWebhookHandler struct {
	svc           *domain.CheckoutService
	webhookSecret string
}

func NewStripeWebhookHandler(svc *domain.CheckoutService, webhookSecret string) *StripeWebhookHandler {
	return &StripeWebhookHandler{
		svc:           svc,
		webhookSecret: webhookSecret,
	}
}

type stripeCheckoutSession struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	ClientReferenceID string `json:"client_reference_id"`
	PaymentIntent     string `json:"payment_intent"`
	Mode              string `json:"mode"`
}

func (h *StripeWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	const maxBodyBytes = 65536
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	event, err := webhook.ConstructEvent(body, r.Header.Get("Stripe-Signature"), h.webhookSecret)
	if err != nil {
		http.Error(w, fmt.Sprintf("webhook signature verification failed: %v", err), http.StatusBadRequest)
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		h.handleCheckoutSessionCompleted(w, r, body)
	case "payment_intent.succeeded":
		h.handlePaymentIntentSucceeded(w, r, body)
	case "charge.refunded":
		h.handleChargeRefunded(w, r, body)
	default:
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "received"})
	}
}

func (h *StripeWebhookHandler) handleCheckoutSessionCompleted(w http.ResponseWriter, r *http.Request, body []byte) {
	var ev struct {
		Data struct {
			Object stripeCheckoutSession `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		http.Error(w, "failed to parse checkout session", http.StatusBadRequest)
		return
	}

	cs := ev.Data.Object

	if cs.ClientReferenceID == "" {
		http.Error(w, "missing client_reference_id", http.StatusBadRequest)
		return
	}

	id, err := parseInt64(cs.ClientReferenceID)
	if err != nil {
		http.Error(w, "invalid client_reference_id", http.StatusBadRequest)
		return
	}

	_, err = h.svc.ConfirmStripePayment(r.Context(), kernel.ID(id), cs.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("confirm stripe payment failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "checkout_completed"})
}

func (h *StripeWebhookHandler) handlePaymentIntentSucceeded(w http.ResponseWriter, r *http.Request, body []byte) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "received"})
}

func (h *StripeWebhookHandler) handleChargeRefunded(w http.ResponseWriter, r *http.Request, body []byte) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "received"})
}

func parseInt64(s string) (int64, error) {
	var v int64
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return 0, err
	}
	return v, nil
}
