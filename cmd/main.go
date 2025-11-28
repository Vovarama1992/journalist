package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	ws "github.com/Vovarama1992/journalist/internal/delivery/ws"
	"github.com/Vovarama1992/journalist/internal/domain"
	"github.com/Vovarama1992/journalist/internal/infra"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {

	// --- env ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	// --- postgres (pgxpool) ---
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("failed to connect pgxpool: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("postgres ping failed: %v", err)
	}

	// --- media repo ---
	mediaRepo := infra.NewPostgresMediaRepo(pool)

	// --- STT ---
	stt := infra.NewYandexSTTService()

	// --- media service ---
	mediaService := domain.NewMediaService(mediaRepo, stt)

	// --- WebSocket hub ---
	hub := ws.NewHub()

	// --- router ---
	r := chi.NewRouter()

	// WebSocket endpoint
	r.Get("/ws", ws.WSHandler(hub, mediaService))

	// healthcheck
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// --- server ---
	addr := ":" + port
	log.Println("listening at", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
