package ports

import (
	"context"

	"github.com/Vovarama1992/journalist/internal/models"
)

type ChunkEvent struct {
	RoomID      string
	MediaID     int
	ChunkNumber int
	Text        string
}

type MediaService interface {
	ProcessMedia(ctx context.Context, sourceURL, mediaType, roomID string) (*models.Media, error)
	Events() <-chan ChunkEvent
}
