package middleware

import (
	"crypto/subtle"
	"net/http"
)

// AdminAuth returns middleware that requires the X-Admin-Token header to match
// the configured admin token. Fails closed: if adminToken is empty (unset), every
// request is rejected regardless of the header, so a forgotten ZAAS_ADMIN_TOKEN
// never silently reopens the gated endpoint to the public.
func AdminAuth(adminToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get("X-Admin-Token")
			if adminToken == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(adminToken)) != 1 {
				writeProblem(r, w, "INVALID_ADMIN_TOKEN",
					"Missing or invalid X-Admin-Token header.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
