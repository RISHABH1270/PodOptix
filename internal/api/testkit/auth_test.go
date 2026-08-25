package testkit

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuth(t *testing.T) {

	t.Run("POST /auth/register", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			w := do(t, http.MethodPost, "/auth/register", `{"email":"register@podoptix.io","password":"secret123"}`, "")
			assert.Equal(t, http.StatusCreated, w.Code)
			assert.Contains(t, w.Body.String(), "token")
			assert.Contains(t, w.Body.String(), "register@podoptix.io")
		})

		t.Run("duplicate email returns 409", func(t *testing.T) {
			body := `{"email":"duplicate@podoptix.io","password":"secret123"}`
			do(t, http.MethodPost, "/auth/register", body, "")
			w := do(t, http.MethodPost, "/auth/register", body, "")
			assert.Equal(t, http.StatusConflict, w.Code)
			assert.Contains(t, w.Body.String(), "already exists")
		})

		t.Run("missing password returns 400", func(t *testing.T) {
			w := do(t, http.MethodPost, "/auth/register", `{"email":"missing@podoptix.io"}`, "")
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	})

	t.Run("POST /auth/login", func(t *testing.T) {
		// seed a user for login tests
		do(t, http.MethodPost, "/auth/register", `{"email":"login@podoptix.io","password":"secret123"}`, "")

		t.Run("success returns token", func(t *testing.T) {
			w := do(t, http.MethodPost, "/auth/login", `{"email":"login@podoptix.io","password":"secret123"}`, "")
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), "token")
		})

		t.Run("wrong password returns 401", func(t *testing.T) {
			w := do(t, http.MethodPost, "/auth/login", `{"email":"login@podoptix.io","password":"wrongpassword"}`, "")
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Contains(t, w.Body.String(), "Invalid email or password")
		})

		t.Run("unknown email returns 401", func(t *testing.T) {
			w := do(t, http.MethodPost, "/auth/login", `{"email":"ghost@podoptix.io","password":"secret123"}`, "")
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Contains(t, w.Body.String(), "Invalid email or password")
		})
	})

	t.Run("JWT middleware", func(t *testing.T) {
		t.Run("no token returns 401", func(t *testing.T) {
			w := do(t, http.MethodGet, "/api/v1/clusters", "", "")
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Contains(t, w.Body.String(), "Authorization header is required")
		})

		t.Run("bad format returns 401", func(t *testing.T) {
			w := do(t, http.MethodGet, "/api/v1/clusters", "", "wrongformat")
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Contains(t, w.Body.String(), "Bearer")
		})

		t.Run("tampered token returns 401", func(t *testing.T) {
			w := do(t, http.MethodGet, "/api/v1/clusters", "", "thisisaninvalidtoken")
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Contains(t, w.Body.String(), "Invalid or expired token")
		})

		t.Run("valid token grants access", func(t *testing.T) {
			// register and extract token
			rw := do(t, http.MethodPost, "/auth/register", `{"email":"jwttest@podoptix.io","password":"secret123"}`, "")
			var res map[string]any
			json.Unmarshal(rw.Body.Bytes(), &res)
			tok, ok := res["token"].(string)
			if !ok {
				t.Fatalf("register failed: %s", rw.Body.String())
			}
			w := do(t, http.MethodGet, "/api/v1/clusters", "", tok)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	})
}
