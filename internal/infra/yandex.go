package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/Vovarama1992/journalist/internal/ports"
)

type YandexSTTService struct {
	apiKey string
}

func NewYandexSTTService() ports.STTService {
	key := os.Getenv("YANDEX_SPEECHKIT_API_KEY")
	if key == "" {
		panic("YANDEX_SPEECHKIT_API_KEY not set")
	}
	return &YandexSTTService{apiKey: key}
}

type yandexResponse struct {
	Result string `json:"result"`
}

// Новая сигнатура
func (s *YandexSTTService) Recognize(ctx context.Context, wav []byte) (string, []byte, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://stt.api.cloud.yandex.net/speech/v1/stt:recognize?lang=ru-RU",
		bytes.NewReader(wav),
	)
	if err != nil {
		return "", nil, err
	}

	req.Header.Set("Authorization", "Api-Key "+s.apiKey)
	req.Header.Set("Content-Type", "audio/wav")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("yandex stt request: %w", err)
	}
	defer resp.Body.Close()

	// читаем тело всегда, целиком
	raw, _ := io.ReadAll(resp.Body)

	// пробуем распарсить JSON
	var r yandexResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		// JSON не распарсился — значит Яндекс вернул ошибку текстом
		return "", raw, fmt.Errorf("yandex stt decode: %w", err)
	}

	return r.Result, raw, nil
}
