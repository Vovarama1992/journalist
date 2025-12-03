package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Vovarama1992/go-utils/logger"
	"github.com/Vovarama1992/journalist/internal/delivery"
	ws "github.com/Vovarama1992/journalist/internal/delivery/ws"
	"github.com/Vovarama1992/journalist/internal/domain"
	"github.com/Vovarama1992/journalist/internal/domain/stations"
	"github.com/Vovarama1992/journalist/internal/infra"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func main() {

	// LOGGER
	zcore, _ := zap.NewProduction()
	zl := logger.NewZapLogger(zcore.Sugar())

	// ENV
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

	// POSTGRES
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

	// SERVICES
	authService := domain.NewAuthService(pool, secret)

	mediaRepo := infra.NewPostgresMediaRepo(pool)
	hMedia := delivery.NewMediaHandler(mediaRepo, zl)
	stt := infra.NewYandexSTTService()

	// GPT CLIENT
	gptClient := infra.NewGPTClient()

	// STATIONS
	s1 := stations.NewS1ResolveURL()
	s2 := stations.NewS2GrabPCM()
	s3 := stations.NewS3PCMtoWAV()
	s4 := stations.NewS4WAVtoText(stt)

	// MEDIA SERVICE (оркестратор)
	mediaService := domain.NewMediaService(
		mediaRepo,
		s1, s2, s3, s4,
		gptClient,
	)

	// WS HUB
	hub := ws.NewHub()

	// BROADCAST LISTENER
	go func() {
		for ev := range mediaService.Events() {

			payload := []byte(
				fmt.Sprintf(`{"mediaId":%d,"chunk":%d,"text":"%s"}`,
					ev.MediaID,
					ev.ChunkNumber,
					ev.Text,
				),
			)

			log.Printf("[SEND] room=%s chunk=%d media=%d text=%.30s",
				ev.RoomID,
				ev.ChunkNumber,
				ev.MediaID,
				ev.Text,
			)

			hub.SendToRoom(ev.RoomID, payload)
		}
	}()

	// HANDLERS
	authHandler := delivery.NewAuthHandler(authService, zl)

	// ROUTER
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "X-Auth"},
		AllowCredentials: true,
	}))

	delivery.RegisterRoutes(r, authHandler, authService, hMedia)

	// WS route
	r.Get("/ws", ws.WSHandler(hub, mediaService))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

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
