package ports

import (
	"context"

	"github.com/Vovarama1992/journalist/internal/models"
)

type MediaRepository interface {
	// EXISTING
	InsertMedia(ctx context.Context, media *models.Media) (*models.Media, error)
	InsertChunk(ctx context.Context, chunk *models.MediaChunk) error
	UpdateChunkText(ctx context.Context, chunkID int, text string) error
	GetLastChunkNumber(ctx context.Context, mediaID int) (int, error)
	GetMediaByID(ctx context.Context, id int) (*models.Media, error)
	GetMediaHistory(ctx context.Context, mediaID int) (string, error)
	GetLastChunk(ctx context.Context, mediaID int) (*models.MediaChunk, error)
	GetLastCompletedChunk(ctx context.Context, mediaID int) (*models.MediaChunk, error)

	// NEW for overlapped ingest
	InsertPendingChunk(ctx context.Context, mediaID int, filePath string) (*models.MediaChunk, error)
	CompleteChunk(
		ctx context.Context,
		mediaID int,
		chunkNumber int,
		text string,
	) error
}
