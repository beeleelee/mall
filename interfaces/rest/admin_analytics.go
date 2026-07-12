package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	dash, err := h.analyticsSvc.GetDashboardOverview(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dash)
}

func (h *AdminHandler) RevenueAnalytics(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	switch r.URL.Query().Get("group") {
	case "product":
		rows, err := h.analyticsSvc.GetRevenueByProduct(r.Context(), limit)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(rows)

	case "category":
		rows, err := h.analyticsSvc.GetRevenueByCategory(r.Context())
		if err != nil {
			writeDomainError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(rows)

	default:
		rows, err := h.analyticsSvc.GetRevenueByDay(r.Context(), days)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		avg, _ := h.analyticsSvc.GetAverageOrderValue(r.Context())
		resp := map[string]any{
			"daily":               rows,
			"average_order_value": avg,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

func (h *AdminHandler) OrderAnalytics(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))

	statusBreakdown, err := h.analyticsSvc.GetOrderStatusBreakdown(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}

	ordersPerDay, err := h.analyticsSvc.GetOrdersPerDay(r.Context(), days)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	cancelRate, err := h.analyticsSvc.GetCancellationRate(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}

	resp := map[string]any{
		"status_breakdown":  statusBreakdown,
		"orders_per_day":    ordersPerDay,
		"cancellation_rate": cancelRate,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *AdminHandler) UserAnalytics(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))

	newUsersPerDay, err := h.analyticsSvc.GetNewUsersPerDay(r.Context(), days)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	statusBreakdown, err := h.analyticsSvc.GetUserStatusBreakdown(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}

	resp := map[string]any{
		"new_users_per_day": newUsersPerDay,
		"status_breakdown":  statusBreakdown,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *AdminHandler) ProductAnalytics(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	topSellers, err := h.analyticsSvc.GetTopSellers(r.Context(), limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	byCategory, err := h.analyticsSvc.GetProductsByCategory(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}

	zeroOrderCount, err := h.analyticsSvc.GetZeroOrderProductCount(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}

	inventorySummary, err := h.analyticsSvc.GetInventorySummary(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}

	resp := map[string]any{
		"top_sellers":        topSellers,
		"by_category":        byCategory,
		"zero_order_count":   zeroOrderCount,
		"inventory_summary":  inventorySummary,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
