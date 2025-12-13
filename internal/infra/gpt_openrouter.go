package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Vovarama1992/journalist/internal/ports"
)

type GPTClient struct {
	apiKey string
	client *http.Client
}

func NewGPTClient() ports.GPTService {
	return &GPTClient{
		apiKey: os.Getenv("OPENROUTER_API_KEY"),
		client: &http.Client{},
	}
}

// sanitize: убираем битый UTF-8
func sanitize(s string) string {
	return strings.ToValidUTF8(s, "")
}

type orMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type orRequest struct {
	Model     string      `json:"model"`
	Messages  []orMessage `json:"messages"`
	MaxTokens int         `json:"max_tokens"`
}

type orResponse struct {
	Choices []struct {
		Message orMessage `json:"message"`
	} `json:"choices"`
}

func (g *GPTClient) ProcessChunk(ctx context.Context, prev, raw string) (string, error) {
	if g.apiKey == "" {
		return "", fmt.Errorf("no OPENROUTER_API_KEY")
	}

	prev = sanitize(prev)
	raw = sanitize(raw)

	systemPrompt := `У тебя есть два текста:

previous — уже принятый, готовый человеческий текст.
raw — новый сырой ASR-текст.

ВАЖНО:
— Ты НЕ цензор.
— Даже если текст кажется странным, глупым, бессмысленным или «хернёй» —
  это ВСЁ РАВНО ТЕКСТ и его НУЖНО обрабатывать.
— Единственный случай, когда можно ничего не возвращать —
  если raw состоит ТОЛЬКО из шума, треска, обрывков звуков,
  не содержащих речи вообще.

АЛГОРИТМ (СТРОГО):

ШАГ 1. Очеловечивание
— Преврати raw в читаемый человеческий текст.
— Это должен быть цельный фрагмент речи.
— НЕ решай, «имеет ли смысл».
— Если в raw есть речь — результат ОБЯЗАН быть непустым.

ШАГ 2. Сшивка
— Мысленно соедини тексты:
  stitched = previous + new

ШАГ 3. Обрезка
— Полностью удали из stitched весь previous целиком.
— Верни только то, что осталось.

ШАГ 4. Пустота
— Если после обрезки ничего не осталось — верни пустую строку.
— ВО ВСЕХ ОСТАЛЬНЫХ СЛУЧАЯХ верни текст.

ПРАВИЛА:
— previous никогда не переписывай.
— previous никогда не возвращай.
— Не объясняй свои действия.
— Верни один цельный текст.
`

	body := orRequest{
		Model:     "openai/gpt-5.1",
		MaxTokens: 300,
		Messages: []orMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("Previous:\n%s\n\nRaw:\n%s", prev, raw)},
		},
	}

	j, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(
			ctx,
			"POST",
			"https://openrouter.ai/api/v1/chat/completions",
			bytes.NewReader(j),
		)
		if err != nil {
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+g.apiKey)
		req.Header.Set("HTTP-Referer", "https://aifulls.com")
		req.Header.Set("X-Title", "journalist-transcriber")

		resp, err := g.client.Do(req)
		if err != nil {
			continue
		}

		rawResp, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		rawResp = bytes.TrimLeftFunc(rawResp, func(r rune) bool {
			return r == '\n' || r == '\r' || r == ' ' || r == '\t'
		})
		if len(rawResp) == 0 {
			continue
		}

		var out orResponse
		if err := json.Unmarshal(rawResp, &out); err != nil {
			continue
		}

		if len(out.Choices) == 0 {
			continue
		}

		return out.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("gpt failed after retries")
}
