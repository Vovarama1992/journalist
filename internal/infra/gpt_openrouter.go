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

previous — это КОНЕЦ уже отображаемого текста на фронтенде
(последнее слово, фраза, предложение ИЛИ специальный fallback-блок).
raw — новый сырой ASR-текст.

ВАЖНО:
— Ты НЕ переписываешь и НЕ пересобираешь прошлый текст.
— Ты НЕ цензор и НЕ оцениваешь «качество» или «смысл».
— Даже странный, кривой или обрывочный текст — это ТЕКСТ.
— ЗАПРЕЩЕНО возвращать пустую строку, если raw содержит речь.
— previous может быть обычным текстом ИЛИ fallback-блоком с многоточием.
— Если previous — fallback-блок, это НЕ причина молчать.

ПРИОРИТЕТЫ (СТРОГО):
1) Очеловечивание raw.
2) Совместимость с previous.
Очеловечивание ВСЕГДА важнее.

ЧТО ТЫ ДЕЛАЕШЬ:

1) Очеловечивание
— Преврати raw в читаемый человеческий текст.
— НЕ меняй смысл и НЕ перефразируй свободно.
— Если в raw есть РЕЧЬ, результат ОБЯЗАН быть непустым.

2) Продолжение текста
— Если previous — обычный текст:
  • верни фрагмент, который логично ПРОДОЛЖАЕТ previous.
  • разрешено выбрать регистр букв и перенос строки.
— Если previous — fallback-блок с многоточием:
  • НЕ пытайся с ним состыковываться,
  • просто верни очеловеченный результат raw.

3) Перекрытие
— Удаляй ТОЛЬКО прямое текстовое перекрытие
  (буквальное или почти буквальное повторение конца previous).
— Смысловое сходство НЕ является перекрытием.
— Если raw содержит речь — текст НУЖНО вернуть.

4) Fallback (ИСПОЛЬЗУЙ ТОЛЬКО ЕСЛИ НЕЛЬЗЯ ИНАЧЕ)
— Если raw содержит речь, но после удаления ПРЯМОГО перекрытия
  невозможно вернуть ни одного слова,
  верни следующий формат:

\n\n...
(КРАТКОЕ ОБЪЯСНЕНИЕ ПРИЧИНЫ: перекрытие, обрыв фразы, шум, и т.п.)

— Причину формулируй по фактической ситуации.
— Если previous уже был fallback-блоком,
  это НЕ запрещает вернуть нормальный текст при наличии речи.

ПРАВИЛА:
— previous никогда не переписывай.
— previous никогда не возвращай.
— Не объясняй свои действия вне указанного формата.
— Верни один цельный фрагмент текста.
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
