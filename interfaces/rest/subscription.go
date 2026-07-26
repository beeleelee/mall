package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"github.com/beeleelee/mall/application/subscription"
	"github.com/beeleelee/mall/domain/kernel"
)

type SubscriptionHandler struct {
	svc *subscription.SubscriptionAppService
}

func NewSubscriptionHandler(svc *subscription.SubscriptionAppService) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc}
}

type createPlanJSON struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Amount        int64    `json:"amount"`
	Interval      string   `json:"interval"`
	IntervalCount int      `json:"interval_count"`
	TrialDays     int      `json:"trial_days"`
	Features      []string `json:"features,omitempty"`
}

type subscribeJSON struct {
	PlanID int64 `json:"plan_id"`
}

type changePlanJSON struct {
	NewPlanID int64 `json:"new_plan_id"`
}

func (h *SubscriptionHandler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var req createPlanJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid request body"))
		return
	}
	resp, err := h.svc.CreatePlan(r.Context(), subscription.CreatePlanRequest{
		Name:          req.Name,
		Description:   req.Description,
		Amount:        req.Amount,
		Interval:      req.Interval,
		IntervalCount: req.IntervalCount,
		TrialDays:     req.TrialDays,
		Features:      req.Features,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *SubscriptionHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.svc.ListPlans(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if plans == nil {
		plans = []*subscription.PlanResponse{}
	}
	writeJSON(w, http.StatusOK, plans)
}

func (h *SubscriptionHandler) GetPlan(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid plan id"))
		return
	}
	plan, err := h.svc.GetPlan(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *SubscriptionHandler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid plan id"))
		return
	}
	var req createPlanJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid request body"))
		return
	}
	resp, err := h.svc.UpdatePlan(r.Context(), id, subscription.CreatePlanRequest{
		Name:          req.Name,
		Description:   req.Description,
		Amount:        req.Amount,
		Interval:      req.Interval,
		IntervalCount: req.IntervalCount,
		TrialDays:     req.TrialDays,
		Features:      req.Features,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *SubscriptionHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	var req subscribeJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid request body"))
		return
	}
	resp, err := h.svc.Subscribe(r.Context(), userID.Int64(), subscription.SubscribeRequest{PlanID: req.PlanID})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *SubscriptionHandler) ListUserSubscriptions(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	subs, err := h.svc.ListUserSubscriptions(r.Context(), userID.Int64())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if subs == nil {
		subs = []*subscription.SubscriptionResponse{}
	}
	writeJSON(w, http.StatusOK, subs)
}

func (h *SubscriptionHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid subscription id"))
		return
	}
	sub, err := h.svc.GetSubscription(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if sub.UserID != userID.Int64() {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrPermissionDenied, "subscription does not belong to user"))
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

func (h *SubscriptionHandler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid subscription id"))
		return
	}
	sub, err := h.svc.GetSubscription(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if sub.UserID != userID.Int64() {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrPermissionDenied, "subscription does not belong to user"))
		return
	}
	cancelled, err := h.svc.CancelSubscription(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cancelled)
}

func (h *SubscriptionHandler) ChangePlan(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid subscription id"))
		return
	}
	var req changePlanJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid request body"))
		return
	}
	sub, err := h.svc.GetSubscription(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if sub.UserID != userID.Int64() {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrPermissionDenied, "subscription does not belong to user"))
		return
	}
	updated, err := h.svc.ChangePlan(r.Context(), id, req.NewPlanID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
