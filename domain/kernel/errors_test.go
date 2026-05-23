package kernel

import (
	"errors"
	"testing"
)

func TestDomainError(t *testing.T) {
	err := NewDomainError(ErrNotFound, "product not found")
	if !IsNotFound(err) {
		t.Error("expected IsNotFound to be true")
	}
	if IsAlreadyExists(err) {
		t.Error("expected IsAlreadyExists to be false")
	}
	if !errors.Is(err, err) {
		t.Error("expected errors.Is to match")
	}
}

func TestDomainError_Wrap(t *testing.T) {
	inner := NewDomainError(ErrConflict, "version conflict")
	outer := NewDomainErrorWithCause(ErrInternal, "checkout failed", inner)

	if !IsConflict(inner) {
		t.Error("inner should be conflict")
	}
	if !IsInternal(outer) {
		t.Error("outer should be internal")
	}
}
