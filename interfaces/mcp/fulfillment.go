package mcp

import (
	"context"
	"encoding/json"

	"github.com/beeleelee/mall/domain/kernel"

	domain "github.com/beeleelee/mall/domain/fulfillment"
)

type FulfillmentMCPHandler struct {
	svc domain.RateCalculator
}

func NewFulfillmentMCPHandler(svc domain.RateCalculator) *FulfillmentMCPHandler {
	return &FulfillmentMCPHandler{svc: svc}
}

var fulfillmentTools = []ToolDefinition{
	{
		Name:        "calculate_rates",
		Description: "Calculate shipping rates for a destination",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"country": {Type: "string", Description: "Destination country code (e.g. US)"},
				"state":   {Type: "string", Description: "Destination state (optional)"},
				"city":    {Type: "string", Description: "Destination city (optional)"},
			},
		},
	},
}

func (h *FulfillmentMCPHandler) ListTools() []ToolDefinition {
	return fulfillmentTools
}

func (h *FulfillmentMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "calculate_rates":
		return h.callCalculateRates(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type calculateRatesArgs struct {
	Country string `json:"country"`
	State   string `json:"state,omitempty"`
	City    string `json:"city,omitempty"`
}

func (h *FulfillmentMCPHandler) callCalculateRates(ctx context.Context, raw json.RawMessage) (any, error) {
	var args calculateRatesArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.Country == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "country must not be empty")
	}

	result, err := h.svc.CalculateRates(ctx, domain.RateInput{
		DestinationCountry: args.Country,
		DestinationState:   args.State,
		DestinationCity:    args.City,
	})
	if err != nil {
		return nil, err
	}

	options := make([]map[string]any, len(result.Options))
	for i, opt := range result.Options {
		options[i] = map[string]any{
			"id":        opt.ID,
			"name":      opt.Name,
			"cost":      opt.Cost,
			"estimated": opt.Estimated,
			"carrier":   opt.Carrier,
		}
	}

	return map[string]any{
		"options": options,
	}, nil
}
