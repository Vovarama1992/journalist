package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Vovarama1992/journalist/internal/ports"
)

type YandexSTTService struct {
	ApiKey string
	URL    string
	Client *http.Client
}

func NewYandexSTTService(apiKey string) ports.STTService {
	return &YandexSTTService{
		ApiKey: apiKey,
		URL:    "https://stt.api.cloud.yandex.net/speech/v1/stt:recognize",
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (y *YandexSTTService) Recognize(ctx context.Context, audio []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", y.URL, bytes.NewReader(audio))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Api-Key "+y.ApiKey)
	req.Header.Set("Content-Type", "audio/ogg") // или другой формат

	resp, err := y.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("yandex stt error: %s", string(body))
	}

	var result struct {
		Result string `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Result, nil
}
