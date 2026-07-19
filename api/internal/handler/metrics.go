package handler

import (
	"net/http"
)

// MetricsHandler returns the Prometheus scrape handler if non-nil,
// otherwise a 404 (metrics disabled).
func MetricsHandler(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	}
}
