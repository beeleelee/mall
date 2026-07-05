package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/jmoiron/sqlx"
	"github.com/zeromicro/go-zero/rest/pathvar"

	"github.com/beeleelee/mall/domain/kernel"

	appidentity "github.com/beeleelee/mall/application/identity"
	catalogdomain "github.com/beeleelee/mall/domain/catalog"
	inventorydomain "github.com/beeleelee/mall/domain/inventory"
	orderdomain "github.com/beeleelee/mall/domain/order"
)

type AdminHandler struct {
	catalogSvc   *catalogdomain.CatalogService
	orderSvc     *orderdomain.OrderService
	identitySvc  *appidentity.IdentityAppService
	inventorySvc *inventorydomain.InventoryService
	storageSvc   kernel.StorageService
	sf           *kernel.Snowflake
	db           *sqlx.DB
}

func NewAdminHandler(catalogSvc *catalogdomain.CatalogService, orderSvc *orderdomain.OrderService, identitySvc *appidentity.IdentityAppService, inventorySvc *inventorydomain.InventoryService, storageSvc kernel.StorageService, sf *kernel.Snowflake, db *sqlx.DB) *AdminHandler {
	return &AdminHandler{
		catalogSvc:   catalogSvc,
		orderSvc:     orderSvc,
		identitySvc:  identitySvc,
		inventorySvc: inventorySvc,
		storageSvc:   storageSvc,
		sf:           sf,
		db:           db,
	}
}

type createProductRequest struct {
	SKU         string         `json:"sku"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	PriceAmount int64          `json:"price_amount"`
	Currency    string         `json:"currency"`
	Attributes  map[string]any `json:"attributes,omitempty"`
}

type updateProductRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	PriceAmount int64          `json:"price_amount"`
	Currency    string         `json:"currency"`
	Status      string         `json:"status,omitempty"`
	Attributes  map[string]any `json:"attributes,omitempty"`
}

func (h *AdminHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req createProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	pid, err := h.sf.NextID()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	product, err := h.catalogSvc.CreateProduct(r.Context(), pid,
		catalogdomain.SKU(req.SKU), req.Name, req.Description, req.Category,
		catalogdomain.Money{Amount: req.PriceAmount, Currency: req.Currency},
		req.Attributes)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(buildProductResponse(product))
}

func (h *AdminHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid product id"))
		return
	}

	var req updateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	product, err := h.catalogSvc.UpdateProduct(r.Context(), kernel.ID(id),
		req.Name, req.Description, req.Category,
		catalogdomain.Money{Amount: req.PriceAmount, Currency: req.Currency},
		catalogdomain.ProductStatus(req.Status), req.Attributes)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(buildProductResponse(product))
}

func (h *AdminHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid product id"))
		return
	}

	if err := h.catalogSvc.DeleteProduct(r.Context(), kernel.ID(id)); err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	orders, err := h.orderSvc.ListAllOrders(r.Context(), offset, limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	resp := make([]orderResponse, len(orders))
	for i, o := range orders {
		resp[i] = buildOrderResponse(o)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	users, err := h.identitySvc.ListUsers(r.Context(), offset, limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(users)
}

func (h *AdminHandler) ActivateUser(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid user id"))
		return
	}

	user, err := h.identitySvc.ActivateUser(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)
}

type setStockRequest struct {
	ProductID         int64 `json:"product_id"`
	Quantity          int   `json:"quantity"`
	LowStockThreshold int   `json:"low_stock_threshold,omitempty"`
}

func (h *AdminHandler) SetStock(w http.ResponseWriter, r *http.Request) {
	var req setStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	_, err := h.inventorySvc.GetStock(r.Context(), kernel.ID(req.ProductID))
	if err == nil {
		item, err := h.inventorySvc.UpdateStock(r.Context(), kernel.ID(req.ProductID), req.Quantity)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(item)
		return
	}

	pid, err := h.sf.NextID()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	threshold := req.LowStockThreshold
	if threshold <= 0 {
		threshold = 10
	}

	item, err := h.inventorySvc.SetStock(r.Context(), pid, kernel.ID(req.ProductID), req.Quantity, threshold)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

func (h *AdminHandler) GetStock(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["productId"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid product id"))
		return
	}

	item, err := h.inventorySvc.GetStock(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(item)
}

func (h *AdminHandler) ListLowStock(w http.ResponseWriter, r *http.Request) {
	threshold, _ := strconv.Atoi(r.URL.Query().Get("threshold"))

	items, err := h.inventorySvc.ListLowStock(r.Context(), threshold)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(items)
}

func (h *AdminHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	pidStr := vars["id"]
	pid, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid product id"))
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "failed to parse multipart form"))
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "missing image file"))
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "failed to read image"))
		return
	}

	imgID, err := h.sf.NextID()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	key := fmt.Sprintf("products/%d/%d", pid, imgID.Int64())
	url, err := h.storageSvc.Upload(r.Context(), key, bytes.NewReader(data), contentType)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInternal, "upload failed: "+err.Error()))
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		"INSERT INTO product_images (id, product_id, url, sort_order) VALUES ($1, $2, $3, $4)",
		imgID.Int64(), pid, url, 0)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInternal, "save image record: "+err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"id":  imgID.Int64(),
		"url": url,
	})
}

func (h *AdminHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["imageId"]
	imgID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid image id"))
		return
	}

	var url string
	err = h.db.GetContext(r.Context(), &url, "SELECT url FROM product_images WHERE id = $1", imgID)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrNotFound, "image not found"))
		return
	}

	_, err = h.db.ExecContext(r.Context(), "DELETE FROM product_images WHERE id = $1", imgID)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInternal, "delete image: "+err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}


