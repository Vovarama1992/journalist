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

type MediaProcessor interface {
	Process(ctx context.Context, url, roomID string) (*models.Media, error)
	Events() <-chan ChunkEvent
}
