package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cbenitezpy/retrodash-server/internal/browser"
	"github.com/cbenitezpy/retrodash-server/internal/origins"
	"github.com/cbenitezpy/retrodash-server/internal/stream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

// mockSourceProvider implements SourceProvider for snapshot tests.
type mockSourceProvider struct {
	provider stream.FrameProvider
}

func (m *mockSourceProvider) GetProvider() stream.FrameProvider {
	return m.provider
}

func (m *mockSourceProvider) GetTouchHandler() *browser.TouchHandler {
	return nil
}

func (m *mockSourceProvider) SwitchToOrigin(_ context.Context, _ *origins.Origin) error {
	return nil
}

// mockFrameProvider implements stream.FrameProvider for snapshot tests.
type mockFrameProvider struct {
	ready           bool
	screenshotData  []byte
	screenshotErr   error
	capturedQuality int
}

func (m *mockFrameProvider) IsReady() bool {
	return m.ready
}

func (m *mockFrameProvider) CaptureScreenshot(_ context.Context, quality int) ([]byte, error) {
	m.capturedQuality = quality
	return m.screenshotData, m.screenshotErr
}

func (m *mockFrameProvider) ViewportSize() (int, int) {
	return 1920, 1080
}

// --- Tests ---

func TestSnapshotHandler_MethodNotAllowed(t *testing.T) {
	handlers := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodPost, "/snapshot", nil)
	rec := httptest.NewRecorder()

	handlers.SnapshotHandler(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Contains(t, rec.Body.String(), "Method not allowed")
}

func TestSnapshotHandler_NilSourceSwitcher(t *testing.T) {
	handlers := NewHandlers(nil)
	// sourceSwitcher intentionally not set

	req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
	rec := httptest.NewRecorder()

	handlers.SnapshotHandler(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "Snapshot not available")
}

func TestSnapshotHandler_NilProvider(t *testing.T) {
	handlers := NewHandlers(nil)
	handlers.SetSourceSwitcher(&mockSourceProvider{provider: nil})

	req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
	rec := httptest.NewRecorder()

	handlers.SnapshotHandler(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "Source not ready")
}

func TestSnapshotHandler_ProviderNotReady(t *testing.T) {
	handlers := NewHandlers(nil)
	handlers.SetSourceSwitcher(&mockSourceProvider{
		provider: &mockFrameProvider{ready: false},
	})

	req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
	rec := httptest.NewRecorder()

	handlers.SnapshotHandler(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "Source not ready")
}

func TestSnapshotHandler_CaptureError(t *testing.T) {
	fp := &mockFrameProvider{
		ready:          true,
		screenshotData: nil,
		screenshotErr:  errors.New("chromedp timeout"),
	}
	handlers := NewHandlers(nil)
	handlers.SetSourceSwitcher(&mockSourceProvider{provider: fp})

	req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
	rec := httptest.NewRecorder()

	handlers.SnapshotHandler(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "Capture failed")
}

func TestSnapshotHandler_DefaultQuality(t *testing.T) {
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10} // minimal JPEG header
	fp := &mockFrameProvider{
		ready:          true,
		screenshotData: jpegData,
	}
	handlers := NewHandlers(nil)
	handlers.SetSourceSwitcher(&mockSourceProvider{provider: fp})

	req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
	rec := httptest.NewRecorder()

	handlers.SnapshotHandler(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 85, fp.capturedQuality, "default quality should be 85")
}

func TestSnapshotHandler_LowQuality(t *testing.T) {
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	fp := &mockFrameProvider{
		ready:          true,
		screenshotData: jpegData,
	}
	handlers := NewHandlers(nil)
	handlers.SetSourceSwitcher(&mockSourceProvider{provider: fp})

	req := httptest.NewRequest(http.MethodGet, "/snapshot?quality=low", nil)
	rec := httptest.NewRecorder()

	handlers.SnapshotHandler(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 50, fp.capturedQuality, "low quality should be 50")
}

func TestSnapshotHandler_HappyPath(t *testing.T) {
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	fp := &mockFrameProvider{
		ready:          true,
		screenshotData: jpegData,
	}
	handlers := NewHandlers(nil)
	handlers.SetSourceSwitcher(&mockSourceProvider{provider: fp})

	req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
	rec := httptest.NewRecorder()

	handlers.SnapshotHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache, no-store, must-revalidate", rec.Header().Get("Cache-Control"))
	assert.Equal(t, jpegData, rec.Body.Bytes())
}
