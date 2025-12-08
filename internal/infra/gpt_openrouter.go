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
	key := os.Getenv("OPENROUTER_API_KEY")
	return &GPTClient{
		apiKey: key,
		client: &http.Client{},
	}
}

// sanitize: убираем битый UTF-8, чтобы JSON не ломался
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

	println("[GPT] start")

	if g.apiKey == "" {
		println("[GPT] fail")
		return "", fmt.Errorf("no OPENROUTER_API_KEY")
	}

	// Приводим ввод к валидному UTF-8 (иначе OpenRouter режет JSON и GPT возвращает пусто)
	prev = sanitize(prev)
	raw = sanitize(raw)

	systemPrompt := `Тебе даются два текста:

previous — уже готовый фрагмент.
raw — сырой ASR-текст (грязный, с повторами, шумами и кривыми фразами).

Задача:
— Превратить raw в чистую, нормальную человеческую речь.
— Убрать повторы, шумы, оговорки, дубли слов, сломанные конструкции.
— Сделать так, чтобы итог органично приклеился к previous.
— previous НЕ возвращать.
— Всегда возвращать непустой текст.

Правила:

Если raw — это прямое продолжение того же говорящего:
— Найти максимальное совпадение между концом previous и началом raw.
— Если начало raw даже частично, примерно или смыслово дублирует конец previous — считать это повтором и вырезать полностью.
— Продолжить фрагмент с маленькой буквы.
— Переписать в нормальную речь.

Если raw звучит как новая реплика:
— Начать строго с новой строки.
— Строка 1: СПИКЕР:
— Строка 2: чистый текст.
— Никаких сшиваний с previous.

Формат:
— Только новый чанк.
— Без HTML.
— Один читабельный фрагмент.
`

	body := orRequest{
		Model:     "openai/gpt-5.1",
		MaxTokens: 300,
		Messages: []orMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("Previous:\n%s\n\nRaw:\n%s", prev, raw)},
		},
	}

	j, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx,
		"POST",
		"https://openrouter.ai/api/v1/chat/completions",
		bytes.NewReader(j),
	)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("HTTP-Referer", "https://aifulls.com")
	req.Header.Set("X-Title", "journalist-transcriber")

	resp, err := g.client.Do(req)
	if err != nil {
		println("[GPT] fail")
		return "", err
	}
	defer resp.Body.Close()

	rawResp, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		println("[GPT] fail")
		return "", fmt.Errorf("gpt status %d", resp.StatusCode)
	}

	var out orResponse
	if err := json.Unmarshal(rawResp, &out); err != nil {
		println("[GPT] fail")
		return "", err
	}

	if len(out.Choices) == 0 {
		println("[GPT] fail")
		return "", fmt.Errorf("no choices")
	}

	println("[GPT] ok")

	return out.Choices[0].Message.Content, nil
}
