package api

import (
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/cbenitezpy/retrodash-server/internal/health"
)

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Flush implements http.Flusher.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// LoggingMiddleware logs HTTP requests.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := newResponseWriter(w)

		next.ServeHTTP(rw, r)

		// Don't log every frame for streaming endpoints
		if r.URL.Path == "/stream" && rw.status == http.StatusOK {
			return
		}

		log.Printf(
			"%s %s %d %s",
			r.Method,
			r.URL.Path,
			rw.status,
			time.Since(start),
		)
	})
}

// RecoveryMiddleware recovers from panics.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v\n%s", err, debug.Stack())
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// PrometheusMiddleware counts HTTP requests using the provided Metrics.
// It records the method, path, and response status for each request.
func PrometheusMiddleware(metrics *health.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := newResponseWriter(w)
			next.ServeHTTP(rw, r)
			// Skip counting streaming endpoints to avoid noise
			if r.URL.Path == "/stream" {
				return
			}
			metrics.IncHTTPRequests(r.Method, normalizePath(r.URL.Path), strconv.Itoa(rw.status))
		})
	}
}

// normalizePath collapses dynamic path segments to prevent unbounded Prometheus
// label cardinality. For example, /api/origins/abc-123 becomes /api/origins/{id}
// and /static/js/main.abc123.js becomes /static/*.
func normalizePath(path string) string {
	if strings.HasPrefix(path, "/api/origins/") {
		rest := strings.TrimPrefix(path, "/api/origins/")
		if rest == "" || rest == "allowed-commands" {
			return path
		}
		if strings.HasSuffix(rest, "/connect") {
			return "/api/origins/{id}/connect"
		}
		return "/api/origins/{id}"
	}
	// Web UI hashed assets — avoid per-hash label explosion.
	if strings.HasPrefix(path, "/static/") {
		return "/static/*"
	}
	return path
}

// CORSMiddleware adds CORS headers for local network access.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from any origin (local network use case)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
