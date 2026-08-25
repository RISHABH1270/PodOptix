package testkit

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealth(t *testing.T) {
	t.Run("liveness returns 200", func(t *testing.T) {
		w := do(t, http.MethodGet, "/healthz", "", "")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"status":"ok"`)
	})
}
