package rest

import (
	"encoding/json"
	"net/http"


	domain "github.com/beeleelee/mall/domain/fulfillment"
)

type FulfillmentHandler struct {
	rateCalc domain.RateCalculator
}

func NewFulfillmentHandler(rateCalc domain.RateCalculator) *FulfillmentHandler {
	return &FulfillmentHandler{rateCalc: rateCalc}
}

type rateInputRequest struct {
	DestinationCountry string          `json:"destination_country"`
	DestinationState   string          `json:"destination_state"`
	DestinationCity    string          `json:"destination_city"`
	Items              []rateItemInput `json:"items"`
}

type rateItemInput struct {
	Weight   float64 `json:"weight"`
	Quantity int     `json:"quantity"`
}

func (h *FulfillmentHandler) CalculateRates(w http.ResponseWriter, r *http.Request) {
	var req rateInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	items := make([]domain.RateItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = domain.RateItem{
			Weight:   item.Weight,
			Quantity: item.Quantity,
		}
	}

	input := domain.RateInput{
		DestinationCountry: req.DestinationCountry,
		DestinationState:   req.DestinationState,
		DestinationCity:    req.DestinationCity,
		Items:              items,
	}

	result, err := h.rateCalc.CalculateRates(r.Context(), input)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	json.NewEncoder(w).Encode(result)
}
