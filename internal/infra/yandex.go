package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

type yandexRequest struct {
	Lang string `json:"lang"`
	// формат фиксированный: PCM 16kHz mono
	// мы будем отдавать WAV сразу, оно подходит
}

type yandexResponse struct {
	Result string `json:"result"`
}

func (s *YandexSTTService) Recognize(ctx context.Context, wav []byte) (string, error) {
	reqBody := bytes.NewReader(wav)

	req, err := http.NewRequestWithContext(ctx,
		"POST",
		"https://stt.api.cloud.yandex.net/speech/v1/stt:recognize?lang=ru-RU",
		reqBody,
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Api-Key "+s.apiKey)
	req.Header.Set("Content-Type", "audio/wav")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("yandex stt request: %w", err)
	}
	defer resp.Body.Close()

	var r yandexResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("yandex stt decode: %w", err)
	}

	return r.Result, nil
}
