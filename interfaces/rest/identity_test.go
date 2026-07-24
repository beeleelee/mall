package rest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

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

type fakeTokenRepo struct {
	mu     sync.Mutex
	tokens map[string]*domain.PasswordResetToken
}

func newFakeTokenRepo() *fakeTokenRepo {
	return &fakeTokenRepo{tokens: make(map[string]*domain.PasswordResetToken)}
}

func (f *fakeTokenRepo) Save(_ context.Context, token *domain.PasswordResetToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[token.TokenHash] = token
	return nil
}

func (f *fakeTokenRepo) FindByHash(_ context.Context, hash string) (*domain.PasswordResetToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tokens[hash]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return t, nil
}

func (f *fakeTokenRepo) MarkUsed(_ context.Context, id kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.tokens {
		if t.ID == id {
			t.MarkUsed()
			return nil
		}
	}
	return kernel.NewDomainError(kernel.ErrNotFound, "not found")
}

func (f *fakeTokenRepo) DeleteExpired(_ context.Context) error { return nil }

type identityTestFixture struct {
	handler   *IdentityHandler
	tokenRepo *fakeTokenRepo
}

func newTestIdentityFixture(t *testing.T) *identityTestFixture {
	t.Helper()
	repo := newFakeUserRepo()
	tokenRepo := newFakeTokenRepo()
	logger := fakeLog{}
	domainSvc := domain.NewIdentityService(repo, logger)
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake failed: %v", err)
	}
	appSvc := app.NewIdentityAppService(domainSvc, repo, tokenRepo, logger, sf)
	return &identityTestFixture{
		handler:   NewIdentityHandler(appSvc),
		tokenRepo: tokenRepo,
	}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func TestIdentityHandler_Register_Success(t *testing.T) {
	fx := newTestIdentityFixture(t)
	h := fx.handler
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
	fx := newTestIdentityFixture(t)
	h := fx.handler
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
	fx := newTestIdentityFixture(t)
	h := fx.handler

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Register(rec, req)
}

func TestIdentityHandler_Login_Success(t *testing.T) {
	fx := newTestIdentityFixture(t)
	h := fx.handler

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
	fx := newTestIdentityFixture(t)
	h := fx.handler

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
	fx := newTestIdentityFixture(t)
	h := fx.handler

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
	fx := newTestIdentityFixture(t)
	h := fx.handler
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/999", nil)
	rec := httptest.NewRecorder()
	h.GetUser(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestIdentityHandler_GetUser_InvalidID(t *testing.T) {
	fx := newTestIdentityFixture(t)
	h := fx.handler
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/abc", nil)
	rec := httptest.NewRecorder()
	h.GetUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func registerUserForReset(h *IdentityHandler, t *testing.T, email string) {
	t.Helper()
	body := map[string]string{"email": email, "password": "password123", "name": "User"}
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	h.Register(httptest.NewRecorder(), req)
}

func TestIdentityHandler_RequestPasswordReset_Success(t *testing.T) {
	fx := newTestIdentityFixture(t)
	h := fx.handler

	regBody := map[string]string{"email": "reset-test@example.com", "password": "password123", "name": "User"}
	regData, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regData))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	h.Register(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register expected 201, got %d: %s", regRec.Code, regRec.Body.String())
	}

	body := map[string]string{"email": "reset-test@example.com"}
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.RequestPasswordReset(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["token"] == "" {
		t.Error("expected non-empty token")
	}
}

func TestIdentityHandler_RequestPasswordReset_NotFound(t *testing.T) {
	fx := newTestIdentityFixture(t)
	h := fx.handler

	body := map[string]string{"email": "nonexistent@example.com"}
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.RequestPasswordReset(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["token"] != "" {
		t.Error("expected empty token for unknown email")
	}
}

func TestIdentityHandler_RequestPasswordReset_InvalidBody(t *testing.T) {
	fx := newTestIdentityFixture(t)
	h := fx.handler

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.RequestPasswordReset(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestIdentityHandler_ResetPassword_Success(t *testing.T) {
	fx := newTestIdentityFixture(t)
	h := fx.handler
	registerUserForReset(h, t, "reset-full@example.com")

	reqBody := map[string]string{"email": "reset-full@example.com"}
	data, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.RequestPasswordReset(rec, req)

	var reqResp map[string]string
	json.NewDecoder(rec.Body).Decode(&reqResp)
	rawToken := reqResp["token"]
	if rawToken == "" {
		t.Fatal("expected non-empty token for password reset")
	}

	resetBody := map[string]string{"token": rawToken, "new_password": "newSecurePass456"}
	resetData, _ := json.Marshal(resetBody)
	resetReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset", bytes.NewReader(resetData))
	resetReq.Header.Set("Content-Type", "application/json")
	resetRec := httptest.NewRecorder()
	h.ResetPassword(resetRec, resetReq)

	if resetRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resetRec.Code)
	}
	var resetResp map[string]string
	json.NewDecoder(resetRec.Body).Decode(&resetResp)
	if resetResp["status"] != "ok" {
		t.Errorf("expected status ok, got %s", resetResp["status"])
	}
}

func TestIdentityHandler_ResetPassword_InvalidToken(t *testing.T) {
	fx := newTestIdentityFixture(t)
	h := fx.handler

	resetBody := map[string]string{"token": "invalid-raw-token", "new_password": "newSecurePass456"}
	data, _ := json.Marshal(resetBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ResetPassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid token, got %d", rec.Code)
	}
}

func TestIdentityHandler_ResetPassword_InvalidBody(t *testing.T) {
	fx := newTestIdentityFixture(t)
	h := fx.handler

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ResetPassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid body, got %d", rec.Code)
	}
}

func TestIdentityHandler_ResetPassword_AlreadyUsed(t *testing.T) {
	fx := newTestIdentityFixture(t)
	h := fx.handler
	registerUserForReset(h, t, "reset-used@example.com")

	reqBody := map[string]string{"email": "reset-used@example.com"}
	data, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.RequestPasswordReset(rec, req)

	var reqResp map[string]string
	json.NewDecoder(rec.Body).Decode(&reqResp)
	rawToken := reqResp["token"]
	if rawToken == "" {
		t.Fatal("expected non-empty token")
	}

	tok, err := fx.tokenRepo.FindByHash(context.Background(), hashToken(rawToken))
	if err != nil {
		t.Fatalf("find token: %v", err)
	}
	if err := fx.tokenRepo.MarkUsed(context.Background(), tok.ID); err != nil {
		t.Fatalf("mark used: %v", err)
	}

	resetBody := map[string]string{"token": rawToken, "new_password": "anotherPass789"}
	resetData, _ := json.Marshal(resetBody)
	resetReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset", bytes.NewReader(resetData))
	resetReq.Header.Set("Content-Type", "application/json")
	resetRec := httptest.NewRecorder()
	h.ResetPassword(resetRec, resetReq)

	if resetRec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for used token, got %d", resetRec.Code)
	}
}

func TestIdentityHandler_ResetPassword_ExpiredToken(t *testing.T) {
	fx := newTestIdentityFixture(t)
	h := fx.handler
	registerUserForReset(h, t, "reset-expired@example.com")

	reqBody := map[string]string{"email": "reset-expired@example.com"}
	data, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.RequestPasswordReset(rec, req)

	var reqResp map[string]string
	json.NewDecoder(rec.Body).Decode(&reqResp)
	rawToken := reqResp["token"]
	if rawToken == "" {
		t.Fatal("expected non-empty token")
	}

	tok, err := fx.tokenRepo.FindByHash(context.Background(), hashToken(rawToken))
	if err != nil {
		t.Fatalf("find token: %v", err)
	}
	tok.ExpiresAt = time.Now().Add(-1 * time.Hour)

	resetBody := map[string]string{"token": rawToken, "new_password": "newPassExpired"}
	resetData, _ := json.Marshal(resetBody)
	resetReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset", bytes.NewReader(resetData))
	resetReq.Header.Set("Content-Type", "application/json")
	resetRec := httptest.NewRecorder()
	h.ResetPassword(resetRec, resetReq)

	if resetRec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for expired token, got %d", resetRec.Code)
	}
}
