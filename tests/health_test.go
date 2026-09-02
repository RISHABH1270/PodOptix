package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealth(t *testing.T) {
	t.Run("liveness returns 200", func(t *testing.T) {
		track(t)
		resp := do(t, http.MethodGet, "/healthz", "", "")
		body := readBody(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, body, `"status":"ok"`)
	})
}
