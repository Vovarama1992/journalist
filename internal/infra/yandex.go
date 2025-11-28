package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Vovarama1992/journalist/internal/ports"
)

type YandexSTTService struct {
	ApiKey string
	URL    string
	Client *http.Client
}

func NewYandexSTTService() ports.STTService {
	apiKey := os.Getenv("YANDEX_SPEECHKIT_API_KEY")
	if apiKey == "" {
		panic("YANDEX_SPEECHKIT_API_KEY is empty")
	}

	return &YandexSTTService{
		ApiKey: apiKey,
		URL:    "https://stt.api.cloud.yandex.net/speech/v1/stt:recognize",
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

func detectContentType(audio []byte) string {
	// OGG magic: "OggS"
	if len(audio) > 4 && string(audio[:4]) == "OggS" {
		return "audio/ogg"
	}

	// WAV magic: "RIFF....WAVE"
	if len(audio) > 12 && string(audio[:4]) == "RIFF" && string(audio[8:12]) == "WAVE" {
		return "audio/wav"
	}

	// MP3: 0xFF 0xFB or ID3
	if len(audio) > 2 && ((audio[0] == 0xFF && audio[1] == 0xFB) || string(audio[:3]) == "ID3") {
		return "audio/mpeg"
	}

	return "audio/ogg" // fallback
}

func (y *YandexSTTService) Recognize(ctx context.Context, audio []byte) (string, error) {
	contentType := detectContentType(audio)

	req, err := http.NewRequestWithContext(ctx, "POST", y.URL, bytes.NewReader(audio))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Api-Key "+y.ApiKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := y.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		var errObj struct {
			ErrorCode    string `json:"error_code"`
			ErrorMessage string `json:"error_message"`
		}
		_ = json.Unmarshal(body, &errObj)

		if errObj.ErrorMessage != "" {
			return "", fmt.Errorf("yandex stt error: %s (%s)", errObj.ErrorMessage, errObj.ErrorCode)
		}

		return "", fmt.Errorf("yandex stt http %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Result string `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode result: %w", err)
	}

	return result.Result, nil
}
