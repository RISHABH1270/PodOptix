package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRegister — POST /auth/register with valid credentials
func TestRegister(t *testing.T) {
	trackTest(t)
	body := `{"email":"test@podoptix.io","password":"secret123"}`

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "token")
	assert.Contains(t, w.Body.String(), "test@podoptix.io")
}

// TestRegister_DuplicateEmail — POST /auth/register same email twice → 409
func TestRegister_DuplicateEmail(t *testing.T) {
	trackTest(t)
	body := `{"email":"duplicate@podoptix.io","password":"secret123"}`

	// first register
	r1 := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
	r1.Header.Set("Content-Type", "application/json")
	testServer.router.ServeHTTP(httptest.NewRecorder(), r1)

	// second register with same email
	r2 := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
	r2.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, r2)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "already exists")
}

// TestRegister_MissingFields — POST /auth/register without password → 400
func TestRegister_MissingFields(t *testing.T) {
	trackTest(t)
	body := `{"email":"missing@podoptix.io"}`

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestLogin — POST /auth/login with correct credentials → 200 + token
func TestLogin(t *testing.T) {
	trackTest(t)

	// register first
	regBody := `{"email":"login@podoptix.io","password":"secret123"}`
	r := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(regBody))
	r.Header.Set("Content-Type", "application/json")
	testServer.router.ServeHTTP(httptest.NewRecorder(), r)

	// login
	body := `{"email":"login@podoptix.io","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "token")
}

// TestLogin_WrongPassword — POST /auth/login with wrong password → 401
func TestLogin_WrongPassword(t *testing.T) {
	trackTest(t)

	regBody := `{"email":"wrongpass@podoptix.io","password":"secret123"}`
	r := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(regBody))
	r.Header.Set("Content-Type", "application/json")
	testServer.router.ServeHTTP(httptest.NewRecorder(), r)

	body := `{"email":"wrongpass@podoptix.io","password":"wrongpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid email or password")
}

// TestLogin_UnknownEmail — POST /auth/login with non-existent email → 401
func TestLogin_UnknownEmail(t *testing.T) {
	trackTest(t)
	body := `{"email":"ghost@podoptix.io","password":"secret123"}`

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid email or password")
}

// TestProtectedRoute_NoToken — GET /api/v1/clusters without token → 401
func TestProtectedRoute_NoToken(t *testing.T) {
	trackTest(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Authorization header is required")
}

// TestProtectedRoute_WrongFormat — GET /api/v1/clusters with bad auth format → 401
func TestProtectedRoute_WrongFormat(t *testing.T) {
	trackTest(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	req.Header.Set("Authorization", "wrongformat")
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Bearer")
}

// TestProtectedRoute_InvalidToken — GET /api/v1/clusters with tampered token → 401
func TestProtectedRoute_InvalidToken(t *testing.T) {
	trackTest(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	req.Header.Set("Authorization", "Bearer thisisaninvalidtoken")
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid or expired token")
}

// TestProtectedRoute_WithToken — GET /api/v1/clusters with valid token → 200
func TestProtectedRoute_WithToken(t *testing.T) {
	trackTest(t)

	// register and get token
	regBody := `{"email":"tokentest@podoptix.io","password":"secret123"}`
	regReq := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regW := httptest.NewRecorder()
	testServer.router.ServeHTTP(regW, regReq)

	var result map[string]any
	json.Unmarshal(regW.Body.Bytes(), &result)
	token, ok := result["token"].(string)
	if !ok {
		t.Fatalf("register failed: %s", regW.Body.String())
	}

	// call protected route with valid token
	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
