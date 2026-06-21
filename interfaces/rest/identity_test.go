package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	app "github.com/beeleelee/mall/application/identity"
	domain "github.com/beeleelee/mall/domain/identity"
	"github.com/beeleelee/mall/domain/kernel"
)

type fakeUserRepo struct {
	users  map[kernel.ID]*domain.User
	emails map[string]kernel.ID
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		users:  make(map[kernel.ID]*domain.User),
		emails: make(map[string]kernel.ID),
	}
}

func (f *fakeUserRepo) Save(_ context.Context, user *domain.User) error {
	f.users[user.ID] = user
	f.emails[user.Email] = user.ID
	return nil
}

func (f *fakeUserRepo) FindByID(_ context.Context, id kernel.ID) (*domain.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "user not found")
	}
	return u, nil
}

func (f *fakeUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	id, ok := f.emails[email]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "user not found")
	}
	u, ok := f.users[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "user not found")
	}
	return u, nil
}

func (f *fakeUserRepo) FindAll(_ context.Context, offset, limit int) ([]*domain.User, error) {
	result := make([]*domain.User, 0, len(f.users))
	for _, u := range f.users {
		result = append(result, u)
	}
	if offset >= len(result) {
		return []*domain.User{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

func (f *fakeUserRepo) Delete(_ context.Context, id kernel.ID) error {
	u, ok := f.users[id]
	if !ok {
		return kernel.NewDomainError(kernel.ErrNotFound, "user not found")
	}
	delete(f.emails, u.Email)
	delete(f.users, id)
	return nil
}

type fakeLog struct{}

func (fakeLog) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLog) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLog) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLog) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}

func newTestIdentityHandler(t *testing.T) *IdentityHandler {
	t.Helper()
	repo := newFakeUserRepo()
	logger := fakeLog{}
	domainSvc := domain.NewIdentityService(repo, logger)
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake failed: %v", err)
	}
	appSvc := app.NewIdentityAppService(domainSvc, repo, logger, sf)
	return NewIdentityHandler(appSvc)
}

func TestIdentityHandler_Register_Success(t *testing.T) {
	h := newTestIdentityHandler(t)
	body := map[string]string{
		"email":    "test@example.com",
		"password": "securepass123",
		"name":     "Test User",
	}
	data, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var resp app.RegisterResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.UserID <= 0 {
		t.Errorf("expected positive UserID, got %d", resp.UserID)
	}
	if resp.Email != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", resp.Email)
	}
}

func TestIdentityHandler_Register_Duplicate(t *testing.T) {
	h := newTestIdentityHandler(t)
	body := map[string]string{
		"email":    "dup@example.com",
		"password": "password123",
		"name":     "User",
	}
	data, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(data))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	h.Register(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate, got %d", rec2.Code)
	}
}

func TestIdentityHandler_Register_InvalidBody(t *testing.T) {
	h := newTestIdentityHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Register(rec, req)
}

func TestIdentityHandler_Login_Success(t *testing.T) {
	h := newTestIdentityHandler(t)

	regBody := map[string]string{
		"email":    "login@example.com",
		"password": "mypassword",
		"name":     "Login User",
	}
	regData, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regData))
	regReq.Header.Set("Content-Type", "application/json")
	h.Register(httptest.NewRecorder(), regReq)

	loginBody := map[string]string{
		"email":    "login@example.com",
		"password": "mypassword",
	}
	loginData, _ := json.Marshal(loginBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginData))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp app.LoginResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Email != "login@example.com" {
		t.Errorf("expected login@example.com, got %s", resp.Email)
	}
}

func TestIdentityHandler_Login_WrongPassword(t *testing.T) {
	h := newTestIdentityHandler(t)

	regBody := map[string]string{
		"email":    "wrongpw@example.com",
		"password": "correctpass",
		"name":     "User",
	}
	regData, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regData))
	regReq.Header.Set("Content-Type", "application/json")
	h.Register(httptest.NewRecorder(), regReq)

	loginBody := map[string]string{
		"email":    "wrongpw@example.com",
		"password": "wrongpass",
	}
	loginData, _ := json.Marshal(loginBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginData))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestIdentityHandler_GetUser_Success(t *testing.T) {
	h := newTestIdentityHandler(t)

	regBody := map[string]string{
		"email":    "getuser@example.com",
		"password": "password123",
		"name":     "Get User",
	}
	regData, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regData))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	h.Register(regRec, regReq)

	var regResp app.RegisterResponse
	json.NewDecoder(regRec.Body).Decode(&regResp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+strconv.FormatInt(regResp.UserID, 10), nil)
	rec := httptest.NewRecorder()
	h.GetUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp app.UserResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != regResp.UserID {
		t.Errorf("expected ID %d, got %d", regResp.UserID, resp.ID)
	}
	if resp.Status != "active" {
		t.Errorf("expected status active, got %s", resp.Status)
	}
}

func TestIdentityHandler_GetUser_NotFound(t *testing.T) {
	h := newTestIdentityHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/999", nil)
	rec := httptest.NewRecorder()
	h.GetUser(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestIdentityHandler_GetUser_InvalidID(t *testing.T) {
	h := newTestIdentityHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/abc", nil)
	rec := httptest.NewRecorder()
	h.GetUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
