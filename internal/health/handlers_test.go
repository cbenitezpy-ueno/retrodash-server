package health

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLivenessHandler(t *testing.T) {
	t.Run("GET returns 200 ok text/plain", func(t *testing.T) {
		handler := LivenessHandler()
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
		assert.Equal(t, "ok", w.Body.String())
	})

	t.Run("POST returns 405 Method Not Allowed", func(t *testing.T) {
		handler := LivenessHandler()
		req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}

func TestReadinessHandler(t *testing.T) {
	t.Run("GET returns 200 ok when provider is ready", func(t *testing.T) {
		provider := &MockStatusProvider{ready: true}
		handler := ReadinessHandler(provider)
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
		assert.Equal(t, "ok", w.Body.String())
	})

	t.Run("GET returns 503 not ready when provider is not ready", func(t *testing.T) {
		provider := &MockStatusProvider{ready: false}
		handler := ReadinessHandler(provider)
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
		assert.Equal(t, "not ready", w.Body.String())
	})

	t.Run("GET returns 503 not ready when provider is nil", func(t *testing.T) {
		handler := ReadinessHandler(nil)
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
		assert.Equal(t, "not ready", w.Body.String())
	})

	t.Run("POST returns 405 Method Not Allowed", func(t *testing.T) {
		provider := &MockStatusProvider{ready: true}
		handler := ReadinessHandler(provider)
		req := httptest.NewRequest(http.MethodPost, "/readyz", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}
