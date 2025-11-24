package delivery

import (
	"net/http"

	"github.com/Vovarama1992/journalist/internal/ports"
)

func AuthMiddleware(auth ports.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// login публичный
			if r.URL.Path == "/api/login" {
				next.ServeHTTP(w, r)
				return
			}

			token := r.Header.Get("X-Auth")
			if token == "" {
				http.Error(w, "missing token", http.StatusUnauthorized)
				return
			}

			ok, _ := auth.ValidateToken(r.Context(), token)
			if !ok {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
