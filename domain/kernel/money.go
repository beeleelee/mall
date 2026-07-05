package kernel

import (
	"fmt"
)

const DefaultCurrency = "USD"

type Money struct {
	Amount   int64
	Currency string
}

func NewMoney(amount int64, currency string) Money {
	if currency == "" {
		currency = DefaultCurrency
	}
	return Money{Amount: amount, Currency: currency}
}

func (m Money) IsZero() bool {
	return m.Amount == 0
}

func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("currency mismatch: %s != %s", m.Currency, other.Currency)
	}
	return Money{Amount: m.Amount + other.Amount, Currency: m.Currency}, nil
}

func (m Money) Sub(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("currency mismatch: %s != %s", m.Currency, other.Currency)
	}
	return Money{Amount: m.Amount - other.Amount, Currency: m.Currency}, nil
}

func (m Money) Multiply(n int) Money {
	return Money{Amount: m.Amount * int64(n), Currency: m.Currency}
}

func (m Money) String() string {
	return fmt.Sprintf("%d.%02d %s", m.Amount/100, m.Amount%100, m.Currency)
}
