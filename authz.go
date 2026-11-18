package counterapi

import (
	"crypto/subtle"
	"net/http"
)

// requireAdmin gates a route on the admin credential: a header whose
// value is the configured token, compared so that the comparison itself
// leaks nothing about how much of it matched. A service started without a
// token refuses the admin route outright.
// orc:ADR-0002#decision.c5
func requireAdmin(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := []byte(r.Header.Get("X-Admin-Token"))
			if token == "" || subtle.ConstantTimeCompare(got, []byte(token)) != 1 {
				httpError(w, http.StatusUnauthorized, "the admin credential is required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
