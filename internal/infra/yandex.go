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
	client *http.Client
}

func NewYandexSTTService() ports.STTService {
	key := os.Getenv("YANDEX_SPEECHKIT_API_KEY")
	if key == "" {
		panic("YANDEX_SPEECHKIT_API_KEY not set")
	}
	return &YandexSTTService{
		apiKey: key,
		client: http.DefaultClient,
	}
}

type yandexResponse struct {
	Result string `json:"result"`
	Error  string `json:"error_message"`
}

func (s *YandexSTTService) Recognize(ctx context.Context, pcm []byte) (string, []byte, error) {

	url := "https://stt.api.cloud.yandex.net/speech/v1/stt:recognize" +
		"?lang=ru-RU&format=lpcm&sampleRateHertz=16000"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(pcm))
	if err != nil {
		return "", nil, err
	}

	req.Header.Set("Authorization", "Api-Key "+s.apiKey)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("yandex stt request: %w", err)
	}
	defer resp.Body.Close()

	rawResp, _ := io.ReadAll(resp.Body)

	// статус проверяем корректно
	if resp.StatusCode != 200 {
		return "", rawResp, fmt.Errorf("yandex stt http %d", resp.StatusCode)
	}

	var parsed yandexResponse
	_ = json.Unmarshal(rawResp, &parsed)

	// ошибка Яндекса
	if parsed.Error != "" {
		return "", rawResp, fmt.Errorf(parsed.Error)
	}

	return parsed.Result, rawResp, nil
}
