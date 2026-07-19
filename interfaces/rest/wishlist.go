package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zeromicro/go-zero/rest/pathvar"

	domain "github.com/beeleelee/mall/domain/wishlist"
	"github.com/beeleelee/mall/domain/kernel"
)

type WishlistHandler struct {
	svc *domain.WishlistService
}

func NewWishlistHandler(svc *domain.WishlistService) *WishlistHandler {
	return &WishlistHandler{svc: svc}
}

type addWishlistItemRequest struct {
	ProductID int64 `json:"product_id"`
}

type wishlistItemResponse struct {
	ProductID int64  `json:"product_id"`
	AddedAt   string `json:"added_at"`
}

type wishlistResponse struct {
	ID     int64                 `json:"id"`
	UserID int64                 `json:"user_id"`
	Items  []wishlistItemResponse `json:"items"`
	Count  int                   `json:"count"`
}

func buildWishlistResponse(w *domain.Wishlist) wishlistResponse {
	items := make([]wishlistItemResponse, len(w.Items))
	for i, item := range w.Items {
		items[i] = wishlistItemResponse{
			ProductID: item.ProductID.Int64(),
			AddedAt:   item.AddedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return wishlistResponse{
		ID:     w.ID.Int64(),
		UserID: w.UserID.Int64(),
		Items:  items,
		Count:  w.ItemCount(),
	}
}

func (h *WishlistHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	wl, err := h.svc.GetWishlist(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(buildWishlistResponse(wl))
}

func (h *WishlistHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	var req addWishlistItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid request body"))
		return
	}

	if err := h.svc.AddItem(r.Context(), userID, kernel.ID(req.ProductID)); err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "added"})
}

func (h *WishlistHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	vars := pathvar.Vars(r)
	productIDStr := vars["productId"]
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid product id"))
		return
	}

	if err := h.svc.RemoveItem(r.Context(), userID, kernel.ID(productID)); err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
}

func (h *WishlistHandler) Clear(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	if err := h.svc.ClearWishlist(r.Context(), userID); err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}
