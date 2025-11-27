package ports

import (
	"context"

	"github.com/Vovarama1992/journalist/internal/models"
)

type MediaService interface {
	// ProcessMedia:
	// 1) Создаёт запись media
	// 2) Нарезает на чанки
	// 3) Сохраняет пустые чанки в БД
	// 4) Отправляет в STT
	// 5) Обновляет чанки текстом
	ProcessMedia(ctx context.Context, sourceURL, mediaType string) (*models.Media, error)
}
