package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zeromicro/go-zero/rest/pathvar"

	domain "github.com/beeleelee/mall/domain/cart"
	"github.com/beeleelee/mall/domain/kernel"
	"github.com/beeleelee/mall/interfaces/middleware"
)

type CartHandler struct {
	svc *domain.CartService
	sf  *kernel.Snowflake
}

func NewCartHandler(svc *domain.CartService, sf *kernel.Snowflake) *CartHandler {
	return &CartHandler{svc: svc, sf: sf}
}

type createCartRequest struct {
	CartID    int64  `json:"cart_id,omitempty"`
	ProductID int64  `json:"product_id,omitempty"`
	SKU       string `json:"sku,omitempty"`
	Name      string `json:"name,omitempty"`
	Quantity  int    `json:"quantity,omitempty"`
	UnitPrice int64  `json:"unit_price,omitempty"`
}

type addItemRequest struct {
	ProductID int64  `json:"product_id"`
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
	ImageURL  string `json:"image_url,omitempty"`
}

type updateQuantityRequest struct {
	Quantity int `json:"quantity"`
}

func userIDFromContext(r *http.Request) (kernel.ID, error) {
	info, ok := middleware.UserFromContext(r.Context())
	if !ok {
		return 0, kernel.NewDomainError(kernel.ErrUnauthenticated, "user not authenticated")
	}
	return kernel.ID(info.UserID), nil
}

func (h *CartHandler) CreateOrGet(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	var req createCartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	cartID := kernel.ID(req.CartID)
	if cartID <= 0 {
		id, err := h.sf.NextID()
		if err != nil {
			writeDomainError(w, err)
			return
		}
		cartID = id
	}

	cart, err := h.svc.GetOrCreateCart(r.Context(), cartID, userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	if req.ProductID > 0 {
		cart, err = h.svc.AddItem(r.Context(), domain.AddItemInput{
			CartID:    cart.ID,
			UserID:    userID,
			ProductID: kernel.ID(req.ProductID),
			SKU:       req.SKU,
			Name:      req.Name,
			Quantity:  req.Quantity,
			UnitPrice: req.UnitPrice,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
	}

	writeCartResponse(w, http.StatusOK, cart)
}

func (h *CartHandler) GetCart(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	cart, err := h.svc.GetCart(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeCartResponse(w, http.StatusOK, cart)
}

func (h *CartHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	vars := pathvar.Vars(r)
	cartIDStr := vars["id"]
	cartID, err := strconv.ParseInt(cartIDStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid cart id"))
		return
	}

	var req addItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	cart, err := h.svc.AddItem(r.Context(), domain.AddItemInput{
		CartID:    kernel.ID(cartID),
		UserID:    userID,
		ProductID: kernel.ID(req.ProductID),
		SKU:       req.SKU,
		Name:      req.Name,
		Quantity:  req.Quantity,
		UnitPrice: req.UnitPrice,
		ImageURL:  req.ImageURL,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeCartResponse(w, http.StatusOK, cart)
}

func (h *CartHandler) UpdateQuantity(w http.ResponseWriter, r *http.Request) {
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

	var req updateQuantityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	cart, err := h.svc.UpdateQuantity(r.Context(), userID, kernel.ID(productID), req.Quantity)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeCartResponse(w, http.StatusOK, cart)
}

func (h *CartHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
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

	cart, err := h.svc.RemoveItem(r.Context(), userID, kernel.ID(productID))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeCartResponse(w, http.StatusOK, cart)
}

func (h *CartHandler) ClearCart(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	cart, err := h.svc.ClearCart(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeCartResponse(w, http.StatusOK, cart)
}

type cartItemResponse struct {
	ProductID int64  `json:"product_id"`
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
	ImageURL  string `json:"image_url,omitempty"`
}

type cartResponse struct {
	ID        int64              `json:"id"`
	UserID    int64              `json:"user_id"`
	Items     []cartItemResponse `json:"items"`
	Status    string             `json:"status"`
	ItemCount int                `json:"item_count"`
	Subtotal  int64              `json:"subtotal"`
}

func writeCartResponse(w http.ResponseWriter, status int, cart *domain.Cart) {
	total := cart.GetTotal()
	items := make([]cartItemResponse, len(cart.Items))
	for i, item := range cart.Items {
		items[i] = cartItemResponse{
			ProductID: item.ProductID.Int64(),
			SKU:       item.SKU,
			Name:      item.Name,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
			ImageURL:  item.ImageURL,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(cartResponse{
		ID:        cart.ID.Int64(),
		UserID:    cart.UserID.Int64(),
		Items:     items,
		Status:    string(cart.Status),
		ItemCount: total.ItemCount,
		Subtotal:  total.Subtotal,
	})
}
