package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/Vovarama1992/go-utils/logger"
	"github.com/Vovarama1992/journalist/internal/delivery"
	ws "github.com/Vovarama1992/journalist/internal/delivery/ws"
	"github.com/Vovarama1992/journalist/internal/domain"
	"github.com/Vovarama1992/journalist/internal/infra"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func main() {

	// ----------------------------------------
	// LOGGER
	// ----------------------------------------
	zcore, _ := zap.NewProduction()
	zl := logger.NewZapLogger(zcore.Sugar())

	// ----------------------------------------
	// ENV
	// ----------------------------------------
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		panic("DATABASE_URL is not set")
	}

	secret := os.Getenv("AUTH_SECRET")
	if secret == "" {
		panic("AUTH_SECRET is not set")
	}

	// ----------------------------------------
	// POSTGRES (pgxpool)
	// ----------------------------------------
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		panic("cannot connect pgxpool: " + err.Error())
	}
	defer pool.Close()

	ctxPing, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctxPing); err != nil {
		panic("postgres ping failed: " + err.Error())
	}

	// ----------------------------------------
	// SERVICES
	// ----------------------------------------
	authService := domain.NewAuthService(pool, secret)

	mediaRepo := infra.NewPostgresMediaRepo(pool)
	stt := infra.NewYandexSTTService()
	mediaService := domain.NewMediaService(mediaRepo, stt)

	streamService := domain.NewStreamService(zl)

	// ----------------------------------------
	// HANDLERS
	// ----------------------------------------
	authHandler := delivery.NewAuthHandler(authService, zl)
	streamHandler := delivery.NewStreamHandler(zl, streamService)

	// ----------------------------------------
	// WS HUB
	// ----------------------------------------
	hub := ws.NewHub()

	// ----------------------------------------
	// ROUTER
	// ----------------------------------------
	r := chi.NewRouter()

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "X-Auth"},
		AllowCredentials: true,
	}))

	// REST routes
	delivery.RegisterRoutes(r, authHandler, streamHandler, authService)

	// WS route
	r.Get("/ws", ws.WSHandler(hub, mediaService))

	// healthcheck
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// ----------------------------------------
	// START SERVER
	// ----------------------------------------
	zl.Log(logger.LogEntry{
		Level:   "info",
		Message: "server started",
		Fields:  map[string]any{"port": port},
	})

	if err := http.ListenAndServe(":"+port, r); err != nil {
		zl.Log(logger.LogEntry{
			Level:   "error",
			Message: "server crashed",
			Error:   err,
		})
	}
}
