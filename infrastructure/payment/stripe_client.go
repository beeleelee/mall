package payment

import (
	"fmt"
	"os"

	"github.com/stripe/stripe-go/v74"
	"github.com/stripe/stripe-go/v74/client"
)

type StripeConfig struct {
	SecretKey      string
	WebhookSecret  string
	PublishableKey string
	BaseURL        string
}

func StripeConfigFromEnv() StripeConfig {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return StripeConfig{
		SecretKey:      os.Getenv("STRIPE_SECRET_KEY"),
		WebhookSecret:  os.Getenv("STRIPE_WEBHOOK_SECRET"),
		PublishableKey: os.Getenv("STRIPE_PUBLISHABLE_KEY"),
		BaseURL:        baseURL,
	}
}

func (c StripeConfig) IsEnabled() bool {
	return c.SecretKey != ""
}

type StripeClient struct {
	*client.API
	Config StripeConfig
}

func NewStripeClient(cfg StripeConfig) *StripeClient {
	sc := &client.API{}
	sc.Init(cfg.SecretKey, nil)
	stripe.DefaultLeveledLogger = &stripe.LeveledLogger{
		Level: stripe.LevelError,
	}
	return &StripeClient{API: sc, Config: cfg}
}

type StripePaymentIntentInfo struct {
	ID           string
	ClientSecret string
	Amount       int64
	Currency     string
	Status       string
}

type StripeCheckoutSessionInfo struct {
	ID              string
	URL             string
	Status          string
	PaymentIntentID string
}

type StripeError struct {
	Code    string
	Message string
}

func (e *StripeError) Error() string {
	return fmt.Sprintf("stripe: %s - %s", e.Code, e.Message)
}
