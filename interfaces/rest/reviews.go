package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/review"
	"github.com/beeleelee/mall/interfaces/middleware"
)

type ReviewHandler struct {
	svc *domain.ReviewService
	sf  *kernel.Snowflake
}

func NewReviewHandler(svc *domain.ReviewService, sf *kernel.Snowflake) *ReviewHandler {
	return &ReviewHandler{svc: svc, sf: sf}
}

type createReviewRequest struct {
	Rating  int    `json:"rating"`
	Title   string `json:"title,omitempty"`
	Content string `json:"content,omitempty"`
}

type reviewResponse struct {
	ID        int64  `json:"id"`
	ProductID int64  `json:"product_id"`
	UserID    int64  `json:"user_id"`
	Rating    int    `json:"rating"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func buildReviewResponse(r *domain.Review) reviewResponse {
	return reviewResponse{
		ID:        r.ID.Int64(),
		ProductID: r.ProductID.Int64(),
		UserID:    r.UserID.Int64(),
		Rating:    int(r.Rating),
		Title:     r.Title,
		Content:   r.Content,
		Status:    string(r.Status),
		CreatedAt: r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (h *ReviewHandler) Create(w http.ResponseWriter, r *http.Request) {
	userInfo, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrUnauthenticated, "user not authenticated"))
		return
	}

	vars := pathvar.Vars(r)
	productIDStr := vars["id"]
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid product id"))
		return
	}

	var req createReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid request body"))
		return
	}

	id, err := h.sf.NextID()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	rv, err := h.svc.CreateReview(r.Context(), id, kernel.ID(productID), kernel.ID(userInfo.UserID), req.Rating, req.Title, req.Content)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(buildReviewResponse(rv))
}

func (h *ReviewHandler) Get(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid review id"))
		return
	}

	rv, err := h.svc.GetReview(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(buildReviewResponse(rv))
}

func (h *ReviewHandler) ListByProduct(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	productIDStr := vars["id"]
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid product id"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	opts := domain.ReviewQueryOptions{Limit: limit, Offset: offset}
	if status := r.URL.Query().Get("status"); status != "" {
		opts.Status = domain.ReviewStatus(status)
	}

	result, err := h.svc.GetReviewsByProduct(r.Context(), kernel.ID(productID), opts)
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

	avg, _ := h.svc.GetAverageRating(r.Context(), kernel.ID(productID))
	resp["average_rating"] = avg

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *ReviewHandler) ListByUser(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	userIDStr := vars["id"]
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid user id"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	opts := domain.ReviewQueryOptions{Limit: limit, Offset: offset}
	if status := r.URL.Query().Get("status"); status != "" {
		opts.Status = domain.ReviewStatus(status)
	}

	result, err := h.svc.GetReviewsByUser(r.Context(), kernel.ID(userID), opts)
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

func (h *ReviewHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userInfo, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrUnauthenticated, "user not authenticated"))
		return
	}

	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid review id"))
		return
	}

	rv, err := h.svc.GetReview(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	if rv.UserID.Int64() != userInfo.UserID {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrPermissionDenied, "can only delete your own review"))
		return
	}

	if err := h.svc.DeleteReview(r.Context(), kernel.ID(id)); err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
