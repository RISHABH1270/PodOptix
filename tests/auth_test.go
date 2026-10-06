package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuth(t *testing.T) {

	t.Run("POST /auth/register", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodPost, "/auth/register", `{"email":"register@podoptix.io","password":"secret123"}`, "")
			body := readBody(t, resp)
			assert.Equal(t, http.StatusCreated, resp.StatusCode)
			assert.Contains(t, body, "token")
			assert.Contains(t, body, "register@podoptix.io")
		})

		t.Run("duplicate email returns 409", func(t *testing.T) {
			track(t)
			b := `{"email":"duplicate@podoptix.io","password":"secret123"}`
			do(t, http.MethodPost, "/auth/register", b, "").Body.Close()
			resp := do(t, http.MethodPost, "/auth/register", b, "")
			body := readBody(t, resp)
			assert.Equal(t, http.StatusConflict, resp.StatusCode)
			assert.Contains(t, body, "already exists")
		})

		t.Run("missing password returns 400", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodPost, "/auth/register", `{"email":"missing@podoptix.io"}`, "")
			resp.Body.Close()
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})

		t.Run("case variants of the same email are the same account", func(t *testing.T) {
			track(t)
			// First register with lowercase
			do(t, http.MethodPost, "/auth/register",
				`{"email":"case@podoptix.io","password":"secret123"}`, "").Body.Close()
			// Try again with upper-case — must collide (store normalizes to lowercase)
			resp := do(t, http.MethodPost, "/auth/register",
				`{"email":"CASE@PodOptix.IO","password":"secret123"}`, "")
			body := readBody(t, resp)
			assert.Equal(t, http.StatusConflict, resp.StatusCode)
			assert.Contains(t, body, "already exists")
		})

		t.Run("login accepts case variants of registered email", func(t *testing.T) {
			track(t)
			do(t, http.MethodPost, "/auth/register",
				`{"email":"caselogin@podoptix.io","password":"secret123"}`, "").Body.Close()
			// Same user, mixed-case login
			resp := do(t, http.MethodPost, "/auth/login",
				`{"email":"CaseLogin@PodOptix.IO","password":"secret123"}`, "")
			body := readBody(t, resp)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, body, "token")
		})

		t.Run("short password rejected with 400", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodPost, "/auth/register",
				`{"email":"shortpw@podoptix.io","password":"abc"}`, "")
			body := readBody(t, resp)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Contains(t, body, "8")
		})

		t.Run("malformed email rejected with 400", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodPost, "/auth/register",
				`{"email":"not-an-email","password":"secret123"}`, "")
			body := readBody(t, resp)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Contains(t, body, "valid email")
		})
	})

	t.Run("POST /auth/login", func(t *testing.T) {
		do(t, http.MethodPost, "/auth/register", `{"email":"login@podoptix.io","password":"secret123"}`, "").Body.Close()

		t.Run("success returns token", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodPost, "/auth/login", `{"email":"login@podoptix.io","password":"secret123"}`, "")
			body := readBody(t, resp)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, body, "token")
		})

		t.Run("wrong password returns 401", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodPost, "/auth/login", `{"email":"login@podoptix.io","password":"wrongpassword"}`, "")
			body := readBody(t, resp)
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			assert.Contains(t, body, "Invalid email or password")
		})

		t.Run("unknown email returns 401", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodPost, "/auth/login", `{"email":"ghost@podoptix.io","password":"secret123"}`, "")
			body := readBody(t, resp)
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			assert.Contains(t, body, "Invalid email or password")
		})
	})

	t.Run("JWT middleware", func(t *testing.T) {
		t.Run("no token returns 401", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodGet, "/api/v1/clusters", "", "")
			body := readBody(t, resp)
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			assert.Contains(t, body, "Authorization header is required")
		})

		t.Run("bad format returns 401", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodGet, "/api/v1/clusters", "", "wrongformat")
			body := readBody(t, resp)
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			assert.Contains(t, body, "Bearer")
		})

		t.Run("tampered token returns 401", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodGet, "/api/v1/clusters", "", "Bearer thisisaninvalidtoken")
			body := readBody(t, resp)
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			assert.Contains(t, body, "Invalid or expired token")
		})

		t.Run("valid token grants access", func(t *testing.T) {
			track(t)
			rw := do(t, http.MethodPost, "/auth/register", `{"email":"jwttest@podoptix.io","password":"secret123"}`, "")
			b, _ := io.ReadAll(rw.Body)
			rw.Body.Close()
			var res map[string]any
			json.Unmarshal(b, &res)
			tok, ok := res["token"].(string)
			if !ok {
				t.Fatalf("register failed: %s", string(b))
			}
			resp := do(t, http.MethodGet, "/api/v1/clusters", "", bearer(tok))
			resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	})
}
