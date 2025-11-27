package ports

import (
	"context"
)

// Postgres репозиторий (progress infra)

// STT сервис (Yandex infra)
type STTService interface {
	// Отправить кусок аудио, получить текст
	Recognize(ctx context.Context, audio []byte) (string, error)
}
