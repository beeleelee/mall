package checkout

import (
	"context"
	"fmt"

	domain "github.com/beeleelee/mall/domain/checkout"
)

type MockPaymentHandler struct {
	registry *domain.PaymentHandlerRegistry
}

func NewMockPaymentHandler() *MockPaymentHandler {
	return &MockPaymentHandler{
		registry: domain.NewPaymentHandlerRegistry(),
	}
}

func (h *MockPaymentHandler) ProcessPayment(ctx context.Context, session *domain.CheckoutSession) (string, error) {
	spec := h.registry.FindByName(session.PaymentHandler)
	if spec == nil {
		spec = h.registry.FindByName("mock")
	}

	if spec.RequiresAP2 {
		verifier := domain.NewAP2Verifier()
		if !verifier.VerifyMandate(session.PaymentHandler) {
			return "", fmt.Errorf("ap2 mandate required but not provided")
		}
	}

	return "payment_txn_" + session.ID.String(), nil
}

func (h *MockPaymentHandler) Refund(ctx context.Context, transactionID string) error {
	if transactionID == "" {
		return fmt.Errorf("transaction id must not be empty")
	}
	return nil
}
