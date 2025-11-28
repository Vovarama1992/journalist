package ports

import (
	"context"

	"github.com/Vovarama1992/journalist/internal/models"
)

type ChunkEvent struct {
	MediaID     int
	ChunkNumber int
	Text        string
}

type MediaService interface {
	ProcessMedia(ctx context.Context, sourceURL, mediaType string) (*models.Media, error)

	// Канал, в который сервис пушит готовые текстовые чанки
	Events() <-chan ChunkEvent
}
