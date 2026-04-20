package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// newTestHandler builds a *handler backed by an in-memory fstest.MapFS
// so tests don't depend on what the real embedded Assets contains.
// Mirrors the caching done by newHandlerFromFS so that callers can
// exercise the same request-handling code path.
func newTestHandler(t *testing.T, files map[string]string) *handler {
	t.Helper()
	m := fstest.MapFS{}
	for name, body := range files {
		m[name] = &fstest.MapFile{Data: []byte(body)}
	}
	var indexHTML []byte
	if f, ok := m[indexHTMLPath]; ok {
		indexHTML = f.Data
	}
	return &handler{assets: m, indexHTML: indexHTML}
}

func TestNewHandler_MissingBundle_ReturnsErrBundleMissing(t *testing.T) {
	// Real Assets has only dist/.gitkeep in fresh checkouts (T019).
	// NewHandler must fail fast rather than crash at request time.
	_, err := NewHandler()
	if !errors.Is(err, ErrBundleMissing) {
		t.Fatalf("want ErrBundleMissing, got %v", err)
	}
}

func TestNewHandlerFromFS_ReturnsHandlerWhenIndexPresent(t *testing.T) {
	h, err := newHandlerFromFS(fstest.MapFS{
		"dist/index.html": &fstest.MapFile{Data: []byte("SHELL")},
	}, "dist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h == nil {
		t.Fatal("want non-nil handler")
	}
}

func TestNewHandlerFromFS_ReturnsBundleMissingWhenIndexAbsent(t *testing.T) {
	_, err := newHandlerFromFS(fstest.MapFS{"dist/.gitkeep": &fstest.MapFile{}}, "dist")
	if !errors.Is(err, ErrBundleMissing) {
		t.Fatalf("want ErrBundleMissing, got %v", err)
	}
}

func TestNewHandlerFromFS_ReturnsErrorWhenSubInvalid(t *testing.T) {
	// fs.Sub rejects paths that are not valid fs paths (contain "..",
	// leading "/", etc.). This exercises the error branch that the real
	// embed.FS never triggers in production.
	_, err := newHandlerFromFS(fstest.MapFS{}, "../escape")
	if err == nil {
		t.Fatal("want error for invalid sub path, got nil")
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	h := newTestHandler(t, map[string]string{
		"index.html": "<!doctype html><title>x</title>",
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", rr.Code)
	}
	if got := rr.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("want Allow=GET, HEAD, got %q", got)
	}
}

func TestHandler_RootServesIndex(t *testing.T) {
	h := newTestHandler(t, map[string]string{
		"index.html": "<!doctype html><title>r</title>",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("want text/html, got %q", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != cacheControlNoStore {
		t.Fatalf("want no-store, got %q", got)
	}
	if body := rr.Body.String(); !strings.Contains(body, "<title>r</title>") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandler_HeadMethodDoesNotWriteBody(t *testing.T) {
	h := newTestHandler(t, map[string]string{
		"index.html": "<!doctype html>body-must-not-leak",
	})

	req := httptest.NewRequest(http.MethodHead, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("HEAD must not return body, got %d bytes", rr.Body.Len())
	}
}

func TestHandler_StaticAsset_Served(t *testing.T) {
	h := newTestHandler(t, map[string]string{
		"index.html":                 "doc",
		"static/js/main.abc123.js":   "console.log(1);",
		"static/css/main.abc123.css": "body{}",
	})

	req := httptest.NewRequest(http.MethodGet, "/static/js/main.abc123.js", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Cache-Control"); got != cacheControlImmutableYear {
		t.Fatalf("want immutable cache, got %q", got)
	}
	if body := rr.Body.String(); body != "console.log(1);" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandler_StaticAsset_MissingReturns404(t *testing.T) {
	h := newTestHandler(t, map[string]string{"index.html": "doc"})

	req := httptest.NewRequest(http.MethodGet, "/static/js/nope.js", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rr.Code)
	}
}

func TestHandler_StaticAsset_BlocksTraversal(t *testing.T) {
	h := newTestHandler(t, map[string]string{"index.html": "doc"})

	for _, p := range []string{"/static/../index.html", "/static/..", "/static/../../etc/passwd"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("traversal %q: want 404, got %d", p, rr.Code)
		}
	}
}

func TestHandler_SPAFallback_HTMLAcceptReturnsIndex(t *testing.T) {
	h := newTestHandler(t, map[string]string{"index.html": "SHELL"})

	req := httptest.NewRequest(http.MethodGet, "/settings/quality", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); body != "SHELL" {
		t.Fatalf("want SHELL body, got %q", body)
	}
}

func TestHandler_SPAFallback_JSONAcceptReturns404(t *testing.T) {
	h := newTestHandler(t, map[string]string{"index.html": "SHELL"})

	req := httptest.NewRequest(http.MethodGet, "/not-a-real-route", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rr.Code)
	}
}

func TestHandler_SPAFallback_EmptyAcceptTreatedAsHTML(t *testing.T) {
	h := newTestHandler(t, map[string]string{"index.html": "SHELL"})

	req := httptest.NewRequest(http.MethodGet, "/deep/link", nil)
	// No Accept header
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 for empty Accept, got %d", rr.Code)
	}
}

func TestHandler_ServeIndex_UsesCachedBytes(t *testing.T) {
	// After construction the handler caches index.html bytes and never
	// re-reads from the FS on subsequent requests. Hand-build the
	// handler with a cached blob that no longer exists on the fs, and
	// verify serveIndex still writes the cached body.
	h := &handler{
		assets:    fstest.MapFS{}, // FS is empty, read-from-fs would fail
		indexHTML: []byte("<html>cached</html>"),
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); body != "<html>cached</html>" {
		t.Fatalf("want cached body, got %q", body)
	}
}

func TestWantsHTML(t *testing.T) {
	cases := map[string]bool{
		"":                                       true,
		"text/html":                              true,
		"text/html, application/xhtml+xml":       true,
		"application/json":                       false,
		"application/xhtml+xml, image/png":       false,
		"*/*":                                    true,
		"text/*":                                 true,
		"text/html;q=0.9, application/xml;q=0.8": true,
		"application/json;q=0.9, text/plain;q=0.5": false,
	}
	for accept, want := range cases {
		if got := wantsHTML(accept); got != want {
			t.Errorf("wantsHTML(%q) = %v, want %v", accept, got, want)
		}
	}
}
