package ports

import (
	"context"

	"github.com/Vovarama1992/journalist/internal/models"
)

// Postgres репозиторий (progress infra)
type MediaRepository interface {
	// Создать запись media
	InsertMedia(ctx context.Context, media *models.Media) (*models.Media, error)
	// Добавить новый чанк
	InsertChunk(ctx context.Context, chunk *models.MediaChunk) error
	// Обновить текст чанка
	UpdateChunkText(ctx context.Context, chunkID int, text string) error
	// Получить последний порядковый номер чанка для media
	GetLastChunkNumber(ctx context.Context, mediaID int) (int, error)
}
