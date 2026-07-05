package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zeromicro/go-zero/rest/pathvar"

	domain "github.com/beeleelee/mall/domain/catalog"
	"github.com/beeleelee/mall/domain/kernel"
)

type CatalogHandler struct {
	svc *domain.CatalogService
}

func NewCatalogHandler(svc *domain.CatalogService) *CatalogHandler {
	return &CatalogHandler{svc: svc}
}

type moneyResponse struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type productResponse struct {
	ID          int64          `json:"id"`
	SKU         string         `json:"sku"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Price       moneyResponse  `json:"price"`
	Status      string         `json:"status"`
	Attributes  map[string]any `json:"attributes"`
	CreatedAt   int64          `json:"created_at"`
	UpdatedAt   int64          `json:"updated_at"`
}

type searchResultResponse struct {
	Products   []productResponse `json:"products"`
	NextCursor string            `json:"next_cursor,omitempty"`
	HasMore    bool              `json:"has_more"`
}

func (h *CatalogHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	opts := domain.SearchOptions{
		Category:      q.Get("category"),
		Cursor:        domain.Cursor(q.Get("cursor")),
		FulltextQuery: q.Get("q"),
	}

	if v := q.Get("min_price"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			opts.MinPrice = n
		}
	}
	if v := q.Get("max_price"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			opts.MaxPrice = n
		}
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			opts.Limit = n
		}
	}

	result, err := h.svc.Search(r.Context(), "", opts)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	resp := searchResultResponse{
		Products:   make([]productResponse, len(result.Products)),
		NextCursor: string(result.NextCursor),
		HasMore:    result.HasMore,
	}
	for i, p := range result.Products {
		resp.Products[i] = buildProductResponse(p)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *CatalogHandler) Lookup(w http.ResponseWriter, r *http.Request) {
	sku := r.URL.Query().Get("sku")
	if sku == "" {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "sku query parameter is required"))
		return
	}

	p, err := h.svc.Lookup(r.Context(), domain.SKU(sku))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(buildProductResponse(p))
}

func (h *CatalogHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid product id"))
		return
	}

	p, err := h.svc.GetProduct(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(buildProductResponse(p))
}

func buildProductResponse(p *domain.Product) productResponse {
	return productResponse{
		ID:          p.ID.Int64(),
		SKU:         string(p.SKU),
		Name:        p.Name,
		Description: p.Description,
		Category:    p.Category,
		Price: moneyResponse{
			Amount:   p.Price.Amount,
			Currency: p.Price.Currency,
		},
		Status:     string(p.Status),
		Attributes: p.Attributes,
		CreatedAt:  p.CreatedAt.UnixMilli(),
		UpdatedAt:  p.UpdatedAt.UnixMilli(),
	}
}
