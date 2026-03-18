package health

import "net/http"

// LivenessHandler returns an http.HandlerFunc for the /healthz liveness probe.
// Always returns 200 "ok" — if the server can respond, it's alive.
// MUST NOT check dependencies (browser, network) to avoid cascade restarts.
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

// ReadinessHandler returns an http.HandlerFunc for the /readyz readiness probe.
// Returns 200 "ok" when the StatusProvider reports ready, 503 "not ready" otherwise.
func ReadinessHandler(provider StatusProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		if provider != nil && provider.IsReady() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready"))
		}
	}
}
