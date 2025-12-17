package delivery

import (
	"github.com/Vovarama1992/journalist/internal/ports"
	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, hAuth *AuthHandler, auth ports.AuthService, hMedia *MediaHandler) {

	// login
	r.Post("/api/login", hAuth.Login)

	// media history
	r.Get("/api/media-history/{id}", hMedia.GetHistory)
}
