package ports

import (
	"context"

	"github.com/Vovarama1992/journalist/internal/models"
)

type MediaRepository interface {
	InsertMedia(ctx context.Context, media *models.Media) (*models.Media, error)
	InsertChunk(ctx context.Context, chunk *models.MediaChunk) error
	UpdateChunkText(ctx context.Context, chunkID int, text string) error
	GetLastChunkNumber(ctx context.Context, mediaID int) (int, error)
	GetMediaByID(ctx context.Context, id int) (*models.Media, error)

	// Новый метод: получить последний чанк
	GetLastChunk(ctx context.Context, mediaID int) (*models.MediaChunk, error)
}
