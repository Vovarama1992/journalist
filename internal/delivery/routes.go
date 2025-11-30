package delivery

import (
	"github.com/Vovarama1992/journalist/internal/ports"
	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(
	r chi.Router,
	hAuth *AuthHandler,
	auth ports.AuthService,
) {

	// login (public)
	r.Post("/api/login", hAuth.Login)
}
