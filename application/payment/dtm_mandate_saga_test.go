package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

type fakeMandateSubmitter struct {
	submitted bool
	gid       string
	payload   mandateSagaPayload
	failErr   error
}

func (f *fakeMandateSubmitter) submit(dtmServer, gid, cbURL string, payload mandateSagaPayload) error {
	if f.failErr != nil {
		return f.failErr
	}
	f.submitted = true
	f.gid = gid
	f.payload = payload
	return nil
}

type fakeLogger struct{}

func (fakeLogger) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLogger) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLogger) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLogger) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}

func TestDTMMandateSaga_Execute(t *testing.T) {
	submitter := &fakeMandateSubmitter{}
	saga := NewDTMMandateSagaWithSubmit(submitter.submit, fakeLogger{})

	err := saga.Execute(context.Background(), 100, "tok_abc")
	if err != nil {
		t.Fatal(err)
	}

	if !submitter.submitted {
		t.Fatal("expected saga to be submitted")
	}
	if submitter.payload.MandateID != 100 {
		t.Errorf("expected mandate_id 100, got %d", submitter.payload.MandateID)
	}
	if submitter.payload.Token != "tok_abc" {
		t.Errorf("expected token tok_abc, got %s", submitter.payload.Token)
	}
}

func TestDTMMandateSaga_SubmitFailure(t *testing.T) {
	submitter := &fakeMandateSubmitter{failErr: errors.New("dtm unavailable")}
	saga := NewDTMMandateSagaWithSubmit(submitter.submit, fakeLogger{})

	err := saga.Execute(context.Background(), 200, "tok_xyz")
	if err == nil {
		t.Fatal("expected error on DTM submit failure")
	}
	if submitter.submitted {
		t.Error("expected no submission on configure fail")
	}
}

func TestDTMMandateSaga_GIDFormat(t *testing.T) {
	submitter := &fakeMandateSubmitter{}
	saga := NewDTMMandateSagaWithSubmit(submitter.submit, fakeLogger{})

	_ = saga.Execute(context.Background(), 42, "tok_test")

	expected := "mt-42"
	if submitter.gid != expected {
		t.Errorf("expected gid %q, got %q", expected, submitter.gid)
	}
}
