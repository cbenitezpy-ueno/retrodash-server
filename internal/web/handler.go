// Web UI static handler for feature 053-web-ui-rnw.
//
// Serves a single-page react-native-web bundle embedded into the binary
// via //go:embed. The public URL layout is:
//
//	GET /               -> dist/index.html         (Cache-Control: no-store)
//	GET /static/<path>  -> dist/static/<path>      (Cache-Control: immutable, 1y)
//	GET /<other>        -> dist/index.html IF Accept includes text/html;
//	                       otherwise 404           (SPA client-side routing)
//
// Anything that the bridge already serves (/api/*, /stream, /snapshot,
// /touch, /health, /healthz, /readyz, /metrics) MUST be registered before
// mounting this handler at the catch-all "/" route so that ServeMux's
// longest-match rule keeps them.
package web

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// emptyTime is passed to http.ServeContent so it skips Last-Modified
// handling — the bundle is immutable once embedded.
var emptyTime = time.Time{}

const (
	indexHTMLPath   = "index.html"
	staticURLPrefix = "/static/"
	htmlMediaType   = "text/html"

	cacheControlHeader        = "Cache-Control"
	cacheControlNoStore       = "no-store"
	cacheControlImmutableYear = "public, max-age=31536000, immutable"
)

// ErrBundleMissing indicates the embedded dist/ directory has no real
// bundle yet (only the .gitkeep placeholder). Callers can treat this
// as a development-time 503 rather than a crash.
var ErrBundleMissing = errors.New("web: embedded web bundle is missing index.html — run `npm run web:build`")

// NewHandler returns an http.Handler that serves the embedded web bundle.
// The returned handler is safe to register at the catch-all "/" pattern
// AFTER all more specific routes (/api/..., /stream, /metrics, etc.).
//
// The function fails fast if the bundle is not present. Callers that
// want to run the bridge without a web UI (e.g. CI images built before
// the webpack stage) should inspect the returned error and decide to
// skip the registration. Internally this delegates to newHandlerFromFS
// so the full logic is exercised in unit tests without depending on what
// the real Assets embed currently contains.
func NewHandler() (http.Handler, error) {
	return newHandlerFromFS(Assets, "dist")
}

// newHandlerFromFS builds a handler from any fs.FS rooted at the given
// sub-directory. Split out from NewHandler so every branch (bad sub,
// missing index, happy path) is unit-testable without touching the
// real embed.FS.
//
// index.html is read ONCE here and cached on the handler — the embed.FS
// contents are immutable for the life of the binary, so reopening the
// file on every GET / is wasteful (hot path on every page load + SPA
// fallback hits).
func newHandlerFromFS(root fs.FS, subDir string) (http.Handler, error) {
	sub, err := fs.Sub(root, subDir)
	if err != nil {
		return nil, err
	}
	indexHTML, err := fs.ReadFile(sub, indexHTMLPath)
	if err != nil {
		return nil, ErrBundleMissing
	}
	return &handler{assets: sub, indexHTML: indexHTML}, nil
}

type handler struct {
	assets    fs.FS
	indexHTML []byte
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	urlPath := r.URL.Path

	// /static/<path> -> asset with immutable caching
	if strings.HasPrefix(urlPath, staticURLPrefix) {
		h.serveStaticAsset(w, r, urlPath)
		return
	}

	// Root path -> index.html
	if urlPath == "/" {
		h.serveIndex(w, r)
		return
	}

	// SPA fallback: non-HTML Accept header returns 404 so API clients
	// don't silently receive the HTML shell.
	if !wantsHTML(r.Header.Get("Accept")) {
		http.NotFound(w, r)
		return
	}
	h.serveIndex(w, r)
}

func (h *handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set(cacheControlHeader, cacheControlNoStore)
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write(h.indexHTML)
}

func (h *handler) serveStaticAsset(w http.ResponseWriter, r *http.Request, urlPath string) {
	// Strip leading slash so fs paths are relative, then clean to block
	// traversal tricks like /static/../secret. After cleaning, the path
	// MUST still live under the static/ prefix — anything that escapes
	// the prefix is either traversal or a direct hit on index.html.
	relPath := path.Clean(strings.TrimPrefix(urlPath, "/"))
	if relPath != "static" && !strings.HasPrefix(relPath, "static/") {
		http.NotFound(w, r)
		return
	}

	data, err := fs.ReadFile(h.assets, relPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set(cacheControlHeader, cacheControlImmutableYear)
	http.ServeContent(w, r, relPath, emptyTime, bytes.NewReader(data))
}

// wantsHTML returns true when the Accept header indicates the caller
// expects HTML (browser navigation). Empty Accept also counts as HTML
// so that curl-style requests without explicit Accept still get the
// SPA shell (matches typical browser behavior).
func wantsHTML(accept string) bool {
	if accept == "" {
		return true
	}
	for _, part := range strings.Split(accept, ",") {
		mediaType := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if mediaType == htmlMediaType || mediaType == "*/*" || mediaType == "text/*" {
			return true
		}
	}
	return false
}
