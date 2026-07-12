package payment

import (
	"context"
	"testing"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

func TestPaymentService_RequestAndApprove(t *testing.T) {
	repo := newFakeMandateRepo()
	svc := NewPaymentService(repo, fakeLogger{})

	scope := MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	}

	m, err := svc.RequestMandate(context.Background(), 1, 42, scope)
	if err != nil {
		t.Fatalf("RequestMandate: %v", err)
	}
	if m.Status != MandateStatusRequested {
		t.Fatalf("expected requested, got %s", m.Status)
	}

	m, err = svc.ApproveMandate(context.Background(), 1, "test-sig")
	if err != nil {
		t.Fatalf("ApproveMandate: %v", err)
	}
	if m.Status != MandateStatusApproved {
		t.Fatalf("expected approved, got %s", m.Status)
	}
	if m.Signature != "test-sig" {
		t.Fatalf("expected signature test-sig, got %s", m.Signature)
	}
}

func TestPaymentService_ExecuteAndSettle(t *testing.T) {
	repo := newFakeMandateRepo()
	svc := NewPaymentService(repo, fakeLogger{})

	scope := MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	}
	svc.RequestMandate(context.Background(), 1, 42, scope)
	svc.ApproveMandate(context.Background(), 1, "sig")

	m, err := svc.ExecuteMandate(context.Background(), 1, "tok-1")
	if err != nil {
		t.Fatalf("ExecuteMandate: %v", err)
	}
	if m.Status != MandateStatusExecuted {
		t.Fatalf("expected executed, got %s", m.Status)
	}
	if m.Token != "tok-1" {
		t.Fatalf("expected token tok-1, got %s", m.Token)
	}

	m, err = svc.SettleMandate(context.Background(), 1)
	if err != nil {
		t.Fatalf("SettleMandate: %v", err)
	}
	if m.Status != MandateStatusSettled {
		t.Fatalf("expected settled, got %s", m.Status)
	}
}

func TestPaymentService_CancelMandate(t *testing.T) {
	repo := newFakeMandateRepo()
	svc := NewPaymentService(repo, fakeLogger{})

	scope := MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	}
	svc.RequestMandate(context.Background(), 1, 42, scope)

	m, err := svc.CancelMandate(context.Background(), 1)
	if err != nil {
		t.Fatalf("CancelMandate: %v", err)
	}
	if m.Status != MandateStatusCancelled {
		t.Fatalf("expected cancelled, got %s", m.Status)
	}
}

type fakeTokenValidator struct {
	shouldFail bool
}

func (v *fakeTokenValidator) ValidateToken(_ context.Context, token, provider string) (*TokenValidationResult, error) {
	if v.shouldFail {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "validation failed")
	}
	return &TokenValidationResult{
		Provider: provider,
		Token:    "validated-" + token,
		Expiry:   time.Now().Add(24 * time.Hour),
	}, nil
}

func TestPaymentService_ExchangeWalletToken(t *testing.T) {
	repo := newFakeMandateRepo()
	svc := NewPaymentService(repo, fakeLogger{}, WithWalletTokenValidator(&fakeTokenValidator{}))

	scope := MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	}
	svc.RequestMandate(context.Background(), 1, 42, scope)
	svc.ApproveMandate(context.Background(), 1, "sig")

	m, err := svc.ExchangeWalletToken(context.Background(), 1, "gpay-token", "google_pay", 42)
	if err != nil {
		t.Fatalf("ExchangeWalletToken: %v", err)
	}
	if m.Status != MandateStatusExecuted {
		t.Fatalf("expected executed, got %s", m.Status)
	}
	if m.Token != "validated-gpay-token" {
		t.Fatalf("expected validated token, got %s", m.Token)
	}
}

func TestPaymentService_ExchangeWalletToken_WrongUser(t *testing.T) {
	repo := newFakeMandateRepo()
	svc := NewPaymentService(repo, fakeLogger{}, WithWalletTokenValidator(&fakeTokenValidator{}))

	scope := MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	}
	svc.RequestMandate(context.Background(), 1, 42, scope)
	svc.ApproveMandate(context.Background(), 1, "sig")

	_, err := svc.ExchangeWalletToken(context.Background(), 1, "token", "google_pay", 99)
	if err == nil {
		t.Fatal("expected error for wrong user")
	}
}

func TestPaymentService_ExchangeWalletToken_ValidatorFailure(t *testing.T) {
	repo := newFakeMandateRepo()
	svc := NewPaymentService(repo, fakeLogger{}, WithWalletTokenValidator(&fakeTokenValidator{shouldFail: true}))

	scope := MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	}
	svc.RequestMandate(context.Background(), 1, 42, scope)
	svc.ApproveMandate(context.Background(), 1, "sig")

	_, err := svc.ExchangeWalletToken(context.Background(), 1, "bad-token", "google_pay", 42)
	if err == nil {
		t.Fatal("expected error for validation failure")
	}
}

func TestPaymentService_RefundMandate_Success(t *testing.T) {
	repo := newFakeMandateRepo()
	svc := NewPaymentService(repo, fakeLogger{})

	scope := MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	}
	svc.RequestMandate(context.Background(), 1, 42, scope)
	svc.ApproveMandate(context.Background(), 1, "sig")
	svc.ExecuteMandate(context.Background(), 1, "tok-1")
	svc.SettleMandate(context.Background(), 1)

	m, err := svc.RefundMandate(context.Background(), 1, 5000)
	if err != nil {
		t.Fatalf("RefundMandate: %v", err)
	}
	if m.Status != MandateStatusRefunded {
		t.Fatalf("expected refunded, got %s", m.Status)
	}
}

func TestPaymentService_RefundMandate_NotSettled(t *testing.T) {
	repo := newFakeMandateRepo()
	svc := NewPaymentService(repo, fakeLogger{})

	scope := MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	}
	svc.RequestMandate(context.Background(), 1, 42, scope)
	svc.ApproveMandate(context.Background(), 1, "sig")

	_, err := svc.RefundMandate(context.Background(), 1, 5000)
	if err == nil {
		t.Fatal("expected error for non-settled mandate")
	}
}

func TestPaymentService_RefundMandate_ExceedsAmount(t *testing.T) {
	repo := newFakeMandateRepo()
	svc := NewPaymentService(repo, fakeLogger{})

	scope := MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	}
	svc.RequestMandate(context.Background(), 1, 42, scope)
	svc.ApproveMandate(context.Background(), 1, "sig")
	svc.ExecuteMandate(context.Background(), 1, "tok-1")
	svc.SettleMandate(context.Background(), 1)

	_, err := svc.RefundMandate(context.Background(), 1, 99999)
	if err == nil {
		t.Fatal("expected error for exceeding max amount")
	}
}

func TestPaymentService_ListUserMandates(t *testing.T) {
	repo := newFakeMandateRepo()
	svc := NewPaymentService(repo, fakeLogger{})

	scope := MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	}
	svc.RequestMandate(context.Background(), 1, 42, scope)
	svc.RequestMandate(context.Background(), 2, 42, scope)

	mandates, err := svc.ListUserMandates(context.Background(), 42)
	if err != nil {
		t.Fatalf("ListUserMandates: %v", err)
	}
	if len(mandates) != 2 {
		t.Fatalf("expected 2 mandates, got %d", len(mandates))
	}
}

type fakeMandateRepo struct {
	mandates map[kernel.ID]*Mandate
}

func newFakeMandateRepo() *fakeMandateRepo {
	return &fakeMandateRepo{mandates: make(map[kernel.ID]*Mandate)}
}

func (r *fakeMandateRepo) Save(_ context.Context, m *Mandate) error {
	r.mandates[m.ID] = m
	return nil
}

func (r *fakeMandateRepo) FindByID(_ context.Context, id kernel.ID) (*Mandate, error) {
	m, ok := r.mandates[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "mandate not found")
	}
	return m, nil
}

func (r *fakeMandateRepo) FindByUserID(_ context.Context, userID kernel.ID) ([]*Mandate, error) {
	var result []*Mandate
	for _, m := range r.mandates {
		if m.UserID == userID {
			result = append(result, m)
		}
	}
	return result, nil
}

func (r *fakeMandateRepo) FindActiveByUser(_ context.Context, userID kernel.ID) ([]*Mandate, error) {
	var result []*Mandate
	for _, m := range r.mandates {
		if m.UserID == userID && m.IsActive() {
			result = append(result, m)
		}
	}
	return result, nil
}

func (r *fakeMandateRepo) Delete(_ context.Context, id kernel.ID) error {
	delete(r.mandates, id)
	return nil
}

type fakeLogger struct{}

func (fakeLogger) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLogger) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLogger) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLogger) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}
