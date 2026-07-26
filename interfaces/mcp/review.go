package mcp

import (
	"context"
	"encoding/json"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/review"
)

type ReviewMCPHandler struct {
	svc   *domain.ReviewService
	sf    *kernel.Snowflake
	tools []ToolDefinition
}

func NewReviewMCPHandler(svc *domain.ReviewService, sf *kernel.Snowflake) *ReviewMCPHandler {
	return &ReviewMCPHandler{
		svc:   svc,
		sf:    sf,
		tools: reviewTools,
	}
}

var reviewTools = []ToolDefinition{
	{
		Name:        "create_review",
		Description: "Create a review for a product",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"product_id": {Type: "number", Description: "Product ID"},
				"rating":     {Type: "number", Description: "Rating 1-5"},
				"title":      {Type: "string", Description: "Review title (optional)"},
				"content":    {Type: "string", Description: "Review content (optional)"},
			},
		},
	},
	{
		Name:        "get_review",
		Description: "Get a review by ID",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"review_id": {Type: "number", Description: "Review ID"},
			},
		},
	},
	{
		Name:        "list_product_reviews",
		Description: "List reviews for a product with pagination",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"product_id": {Type: "number", Description: "Product ID"},
				"limit":      {Type: "number", Description: "Max results (max 100)"},
				"offset":     {Type: "number", Description: "Pagination offset"},
			},
		},
	},
	{
		Name:        "list_user_reviews",
		Description: "List reviews by a user with pagination",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id": {Type: "number", Description: "User ID"},
				"limit":   {Type: "number", Description: "Max results (max 100)"},
				"offset":  {Type: "number", Description: "Pagination offset"},
			},
		},
	},
	{
		Name:        "delete_review",
		Description: "Delete a review by ID",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"review_id": {Type: "number", Description: "Review ID to delete"},
			},
		},
	},
	{
		Name:        "get_product_rating",
		Description: "Get the average rating for a product",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"product_id": {Type: "number", Description: "Product ID"},
			},
		},
	},
}

func (h *ReviewMCPHandler) ListTools() []ToolDefinition {
	return h.tools
}

func (h *ReviewMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "create_review":
		return h.callCreate(ctx, raw)
	case "get_review":
		return h.callGet(ctx, raw)
	case "list_product_reviews":
		return h.callListByProduct(ctx, raw)
	case "list_user_reviews":
		return h.callListByUser(ctx, raw)
	case "delete_review":
		return h.callDelete(ctx, raw)
	case "get_product_rating":
		return h.callAverageRating(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type createReviewArgs struct {
	ProductID int64  `json:"product_id"`
	Rating    int    `json:"rating"`
	Title     string `json:"title"`
	Content   string `json:"content"`
}

type reviewIDArgs struct {
	ReviewID int64 `json:"review_id"`
}

type listReviewsArgs struct {
	ProductID int64 `json:"product_id"`
	UserID    int64 `json:"user_id"`
	Limit     int   `json:"limit"`
	Offset    int   `json:"offset"`
}

type productIDArgs struct {
	ProductID int64 `json:"product_id"`
}

func buildReviewResponse(r *domain.Review) map[string]any {
	return map[string]any{
		"id":         r.ID.Int64(),
		"product_id": r.ProductID.Int64(),
		"user_id":    r.UserID.Int64(),
		"rating":     int(r.Rating),
		"title":      r.Title,
		"content":    r.Content,
		"status":     string(r.Status),
		"created_at": r.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func (h *ReviewMCPHandler) callCreate(ctx context.Context, raw json.RawMessage) (any, error) {
	var args createReviewArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ProductID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "product_id is required")
	}

	id, err := h.sf.NextID()
	if err != nil {
		return nil, err
	}

	rv, err := h.svc.CreateReview(ctx, id, kernel.ID(args.ProductID), 0, args.Rating, args.Title, args.Content)
	if err != nil {
		return nil, err
	}
	return buildReviewResponse(rv), nil
}

func (h *ReviewMCPHandler) callGet(ctx context.Context, raw json.RawMessage) (any, error) {
	var args reviewIDArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	rv, err := h.svc.GetReview(ctx, kernel.ID(args.ReviewID))
	if err != nil {
		return nil, err
	}
	return buildReviewResponse(rv), nil
}

func (h *ReviewMCPHandler) callListByProduct(ctx context.Context, raw json.RawMessage) (any, error) {
	var args listReviewsArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	opts := domain.ReviewQueryOptions{Limit: args.Limit, Offset: args.Offset}
	result, err := h.svc.GetReviewsByProduct(ctx, kernel.ID(args.ProductID), opts)
	if err != nil {
		return nil, err
	}
	reviews := make([]map[string]any, len(result.Reviews))
	for i, rv := range result.Reviews {
		reviews[i] = buildReviewResponse(rv)
	}
	return map[string]any{
		"reviews": reviews,
		"total":   result.Total,
	}, nil
}

func (h *ReviewMCPHandler) callListByUser(ctx context.Context, raw json.RawMessage) (any, error) {
	var args listReviewsArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	opts := domain.ReviewQueryOptions{Limit: args.Limit, Offset: args.Offset}
	result, err := h.svc.GetReviewsByUser(ctx, kernel.ID(args.UserID), opts)
	if err != nil {
		return nil, err
	}
	reviews := make([]map[string]any, len(result.Reviews))
	for i, rv := range result.Reviews {
		reviews[i] = buildReviewResponse(rv)
	}
	return map[string]any{
		"reviews": reviews,
		"total":   result.Total,
	}, nil
}

func (h *ReviewMCPHandler) callDelete(ctx context.Context, raw json.RawMessage) (any, error) {
	var args reviewIDArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if err := h.svc.DeleteReview(ctx, kernel.ID(args.ReviewID)); err != nil {
		return nil, err
	}
	return map[string]any{"deleted": true, "review_id": args.ReviewID}, nil
}

func (h *ReviewMCPHandler) callAverageRating(ctx context.Context, raw json.RawMessage) (any, error) {
	var args productIDArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	avg, err := h.svc.GetAverageRating(ctx, kernel.ID(args.ProductID))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"product_id": args.ProductID,
		"average":    avg,
	}, nil
}


