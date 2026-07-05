package kernel

import (
	"testing"
)

func TestMoneyAdd(t *testing.T) {
	a := NewMoney(100, "USD")
	b := NewMoney(200, "USD")

	r, err := a.Add(b)
	if err != nil {
		t.Fatal(err)
	}
	if r.Amount != 300 || r.Currency != "USD" {
		t.Fatalf("expected 300 USD, got %d %s", r.Amount, r.Currency)
	}
}

func TestMoneyAddCurrencyMismatch(t *testing.T) {
	a := NewMoney(100, "USD")
	b := NewMoney(200, "EUR")

	_, err := a.Add(b)
	if err == nil {
		t.Fatal("expected currency mismatch error")
	}
}

func TestMoneySub(t *testing.T) {
	a := NewMoney(300, "USD")
	b := NewMoney(100, "USD")

	r, err := a.Sub(b)
	if err != nil {
		t.Fatal(err)
	}
	if r.Amount != 200 {
		t.Fatalf("expected 200, got %d", r.Amount)
	}
}

func TestMoneyMultiply(t *testing.T) {
	m := NewMoney(150, "USD")
	r := m.Multiply(3)
	if r.Amount != 450 {
		t.Fatalf("expected 450, got %d", r.Amount)
	}
}

func TestMoneyIsZero(t *testing.T) {
	if !NewMoney(0, "USD").IsZero() {
		t.Fatal("expected zero")
	}
	if NewMoney(1, "USD").IsZero() {
		t.Fatal("expected non-zero")
	}
}

func TestMoneyDefaultCurrency(t *testing.T) {
	m := NewMoney(100, "")
	if m.Currency != "USD" {
		t.Fatalf("expected USD, got %s", m.Currency)
	}
}

func TestMoneyString(t *testing.T) {
	m := NewMoney(1050, "USD")
	s := m.String()
	if s != "10.50 USD" {
		t.Fatalf("expected '10.50 USD', got %q", s)
	}
}
