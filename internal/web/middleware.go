package web

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

// BasicAuth returns middleware that enforces HTTP Basic Auth against a stored
// bcrypt hash. The username is ignored; only the password is checked.
func BasicAuth(passwordHash string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Username is intentionally ignored — this app uses a single shared
			// password with no per-user accounts.
			_, password, ok := r.BasicAuth()
			if !ok || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
				w.Header().Set("WWW-Authenticate", `Basic realm="AurelianPRM"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
