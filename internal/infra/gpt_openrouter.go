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
		println("[GPT] fail: no API KEY")
		return "", fmt.Errorf("no OPENROUTER_API_KEY")
	}

	prev = sanitize(prev)
	raw = sanitize(raw)

	// ЛОГИРУЕМ ВХОД
	fmt.Printf("[GPT][IN-prev] %q\n", prev)
	fmt.Printf("[GPT][IN-raw ] %q\n", raw)

	systemPrompt := `Тебе даются два текста:

previous — уже готовый фрагмент.
raw — сырой ASR-текст (грязный, с повторами, шумами и кривыми фразами).

Задача:
— Превратить raw в чистую, нормальную человеческую речь.
— Убрать повторы, шумы, оговорки, дубли слов, сломанные конструкции.
— Сделать так, чтобы итог органично приклеился к previous.
— previous НЕ возвращать.
— Всегда возвращать непустой текст, кроме одного случая: 
  если raw полностью (или почти полностью) повторяет previous — верни пустую строку.

Важное уточнение:
— Если raw НЕ является дублем previous, но является новой мыслью, новой темой или новым фрагментом — 
  НЕЛЬЗЯ возвращать пустую строку. В этих случаях обязательно нужно выдать содержательный текст.

Правила:

Если raw — это прямое продолжение того же говорящего:
— Найти максимальное совпадение между концом previous и началом raw.
— Если начало raw даже частично, примерно или смыслово дублирует конец previous — считать это повтором и вырезать полностью.
— Продолжить фрагмент с маленькой буквой.
— Переписать в нормальную речь.
— Итог НЕ должен быть пустым.

Если raw звучит как новая реплика:
— Начать строго с новой строки.
— Строка 1: СПИКЕР:
— Строка 2: чистый текст.
— Итог НЕ должен быть пустым.

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

	// ЛОГИРУЕМ JSON КОТОРЫЙ ОТПРАВЛЯЕМ
	fmt.Printf("[GPT][REQ] %s\n", j)

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
		fmt.Printf("[GPT][HTTP-ERR] %v\n", err)
		return "", err
	}
	defer resp.Body.Close()

	rawResp, _ := io.ReadAll(resp.Body)

	// ЛОГИРУЕМ ВСЁ, что вернул OpenRouter
	fmt.Printf("[GPT][RESP-CODE] %d\n", resp.StatusCode)
	fmt.Printf("[GPT][RESP-BODY] %s\n", rawResp)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("gpt status %d", resp.StatusCode)
	}

	var out orResponse
	if err := json.Unmarshal(rawResp, &out); err != nil {
		fmt.Printf("[GPT][JSON-ERR] %v\n", err)
		return "", err
	}

	if len(out.Choices) == 0 {
		fmt.Println("[GPT][ERR] no choices")
		return "", fmt.Errorf("no choices")
	}

	fmt.Printf("[GPT][OUT] %q\n", out.Choices[0].Message.Content)
	println("[GPT] ok")

	return out.Choices[0].Message.Content, nil
}
