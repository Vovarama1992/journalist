package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Vovarama1992/go-utils/logger"
	"github.com/Vovarama1992/journalist/internal/delivery"
	"github.com/Vovarama1992/journalist/internal/domain"
	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

func main() {
	// --- logger ---
	base, _ := zap.NewProduction()
	zl := logger.NewZapLogger(base.Sugar())

	// --- env ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	secret := os.Getenv("AUTH_SECRET")
	if secret == "" {
		log.Fatal("AUTH_SECRET is not set")
	}

	// --- db ---
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatalf("db ping failed: %v", err)
	}

	// --- services ---
	authService := domain.NewAuthService(db, secret)

	// --- handlers ---
	authHandler := delivery.NewAuthHandler(authService, zl)
	streamHandler := delivery.NewStreamHandler(zl)

	// --- router ---
	r := chi.NewRouter()
	delivery.RegisterRoutes(r, authHandler, streamHandler, authService)

	// --- server ---
	addr := ":" + port
	log.Println("listening at", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
