package delivery

import (
	"github.com/Vovarama1992/journalist/internal/ports"
	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(
	r chi.Router,
	hAuth *AuthHandler,
	hStream *StreamHandler,
	auth ports.AuthService,
) {

	// login (public)
	r.Post("/api/login", hAuth.Login)

	// protected group
	r.Group(func(pr chi.Router) {
		pr.Use(AuthMiddleware(auth))

		// сюда все защищённые эндпоинты
		pr.Get("/api/start", hStream.Start)
	})
}
