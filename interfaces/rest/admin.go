package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/zeromicro/go-zero/rest/pathvar"

	"github.com/beeleelee/mall/domain/analytics"
	"github.com/beeleelee/mall/domain/kernel"
	reviewdomain "github.com/beeleelee/mall/domain/review"

	appidentity "github.com/beeleelee/mall/application/identity"
	catalogdomain "github.com/beeleelee/mall/domain/catalog"
	inventorydomain "github.com/beeleelee/mall/domain/inventory"
	orderdomain "github.com/beeleelee/mall/domain/order"
)

type AdminHandler struct {
	catalogSvc      *catalogdomain.CatalogService
	orderSvc        *orderdomain.OrderService
	identitySvc     *appidentity.IdentityAppService
	inventorySvc    *inventorydomain.InventoryService
	storageSvc      kernel.StorageService
	categoryRepo    catalogdomain.CategoryRepository
	analyticsSvc    *analytics.AnalyticsService
	reviewSvc       *reviewdomain.ReviewService
	sf              *kernel.Snowflake
	db              *sqlx.DB
	deliveryLogRepo orderdomain.DeliveryLogRepository
	refundSvc       *orderdomain.RefundService
}

func NewAdminHandler(catalogSvc *catalogdomain.CatalogService, orderSvc *orderdomain.OrderService, identitySvc *appidentity.IdentityAppService, inventorySvc *inventorydomain.InventoryService, storageSvc kernel.StorageService, categoryRepo catalogdomain.CategoryRepository, analyticsSvc *analytics.AnalyticsService, reviewSvc *reviewdomain.ReviewService, sf *kernel.Snowflake, db *sqlx.DB, deliveryLogRepo orderdomain.DeliveryLogRepository, refundSvc *orderdomain.RefundService) *AdminHandler {
	return &AdminHandler{
		catalogSvc:      catalogSvc,
		orderSvc:        orderSvc,
		identitySvc:     identitySvc,
		inventorySvc:    inventorySvc,
		storageSvc:      storageSvc,
		categoryRepo:    categoryRepo,
		analyticsSvc:    analyticsSvc,
		reviewSvc:       reviewSvc,
		sf:              sf,
		db:              db,
		deliveryLogRepo: deliveryLogRepo,
		refundSvc:       refundSvc,
	}
}

type createProductRequest struct {
	SKU         string         `json:"sku"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	CategoryID  int64          `json:"category_id,omitempty"`
	PriceAmount int64          `json:"price_amount"`
	Currency    string         `json:"currency"`
	Attributes  map[string]any `json:"attributes,omitempty"`
}

type updateProductRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	CategoryID  int64          `json:"category_id,omitempty"`
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
		kernel.ID(req.CategoryID),
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
		req.Name, req.Description, req.Category, kernel.ID(req.CategoryID),
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

// --- Category CRUD ---

type createCategoryRequest struct {
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	ParentID  int64  `json:"parent_id,omitempty"`
	SortOrder int    `json:"sort_order,omitempty"`
}

type updateCategoryRequest struct {
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	ParentID  int64  `json:"parent_id,omitempty"`
	SortOrder int    `json:"sort_order,omitempty"`
}

func (h *AdminHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req createCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid request body"))
		return
	}

	cid, err := h.sf.NextID()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	cat, err := catalogdomain.NewCategory(cid, req.Name, req.Slug, kernel.ID(req.ParentID), req.SortOrder)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	if err := h.categoryRepo.Save(r.Context(), cat); err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cat)
}

func (h *AdminHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	cid, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid category id"))
		return
	}

	cat, err := h.categoryRepo.FindByID(r.Context(), kernel.ID(cid))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	var req updateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid request body"))
		return
	}

	if err := cat.Update(req.Name, req.Slug, kernel.ID(req.ParentID), req.SortOrder); err != nil {
		writeDomainError(w, err)
		return
	}

	if err := h.categoryRepo.Save(r.Context(), cat); err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(cat)
}

func (h *AdminHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.categoryRepo.FindAll(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(categories)
}

func (h *AdminHandler) GetCategory(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	cid, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid category id"))
		return
	}

	cat, err := h.categoryRepo.FindByID(r.Context(), kernel.ID(cid))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(cat)
}

func (h *AdminHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	cid, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid category id"))
		return
	}

	if err := h.categoryRepo.Delete(r.Context(), kernel.ID(cid)); err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
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

type deliveryLogResponse struct {
	ID        int64  `json:"id"`
	WebhookID int64  `json:"webhook_id"`
	Event     string `json:"event"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	Attempts  int    `json:"attempts"`
	CreatedAt string `json:"created_at"`
}

func (h *AdminHandler) ListFailedDeliveries(w http.ResponseWriter, r *http.Request) {
	entries, err := h.deliveryLogRepo.ListFailed(r.Context(), 50)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	result := make([]deliveryLogResponse, 0, len(entries))
	for _, e := range entries {
		result = append(result, deliveryLogResponse{
			ID:        e.ID,
			WebhookID: e.WebhookID,
			Event:     e.Event,
			Status:    e.Status,
			Error:     e.Error,
			Attempts:  e.Attempts,
			CreatedAt: e.CreatedAt.Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *AdminHandler) RetryDelivery(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	logID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid delivery log id"))
		return
	}

	var webhookID int64
	var eventStr string
	var payload []byte
	err = h.db.QueryRowContext(r.Context(),
		"SELECT webhook_id, event, payload FROM webhook_delivery_log WHERE id = $1 AND status = 'failed'", logID).
		Scan(&webhookID, &eventStr, &payload)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrNotFound, "failed delivery log not found"))
		return
	}

	var url string
	var secret string
	err = h.db.QueryRowContext(r.Context(),
		"SELECT url, secret FROM webhooks WHERE id = $1 AND active = true", webhookID).
		Scan(&url, &secret)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrNotFound, "webhook not found or inactive"))
		return
	}

	signature := orderdomain.SignWebhookPayload(secret, payload)
	timestamp := time.Now().UnixMilli()

	body := map[string]any{
		"event":     eventStr,
		"timestamp": timestamp,
		"payload":   json.RawMessage(payload),
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInternal, "create request: "+err.Error()))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature-256", signature)
	req.Header.Set("X-Signature-Timestamp", fmt.Sprintf("%d", timestamp))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		_ = h.deliveryLogRepo.Save(r.Context(), &orderdomain.DeliveryLogEntry{
			ID:        logID,
			WebhookID: webhookID,
			Event:     eventStr,
			Payload:   payload,
			Status:    "failed",
			Error:     err.Error(),
			Attempts:  0,
		})
		writeDomainError(w, kernel.NewDomainError(kernel.ErrUnavailable, "retry failed: "+err.Error()))
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_ = h.deliveryLogRepo.MarkRetried(r.Context(), logID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "retried"})
	} else {
		errMsg := fmt.Sprintf("webhook returned status %d", resp.StatusCode)
		_ = h.deliveryLogRepo.Save(r.Context(), &orderdomain.DeliveryLogEntry{
			ID:        logID,
			WebhookID: webhookID,
			Event:     eventStr,
			Payload:   payload,
			Status:    "failed",
			Error:     errMsg,
			Attempts:  0,
		})
		writeDomainError(w, kernel.NewDomainError(kernel.ErrUnavailable, errMsg))
	}
}
