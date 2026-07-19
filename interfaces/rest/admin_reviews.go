package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/review"
)

func (h *AdminHandler) ListAllReviews(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	opts := domain.ReviewQueryOptions{Limit: limit, Offset: offset}
	if status := r.URL.Query().Get("status"); status != "" {
		opts.Status = domain.ReviewStatus(status)
	}

	result, err := h.reviewSvc.GetAllReviews(r.Context(), opts)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	reviews := make([]reviewResponse, len(result.Reviews))
	for i, rv := range result.Reviews {
		reviews[i] = buildReviewResponse(rv)
	}

	resp := map[string]any{
		"reviews": reviews,
		"total":   result.Total,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *AdminHandler) ApproveReview(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid review id"))
		return
	}

	rv, err := h.reviewSvc.ApproveReview(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(buildReviewResponse(rv))
}

func (h *AdminHandler) RejectReview(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid review id"))
		return
	}

	rv, err := h.reviewSvc.RejectReview(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(buildReviewResponse(rv))
}
