package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type GPTClient struct {
	apiKey string
	client *http.Client
}

func NewGPTClient() *GPTClient {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		log.Printf("[GPT] FATAL: OPENROUTER_API_KEY is empty")
	}

	return &GPTClient{
		apiKey: key,
		client: &http.Client{},
	}
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

// ProcessChunk — просто реализует ports.GPTService
func (g *GPTClient) ProcessChunk(ctx context.Context, prev, raw string) (string, error) {
	if g.apiKey == "" {
		return "", fmt.Errorf("no OPENROUTER_API_KEY")

	}

	// SYSTEM PROMPT — спокойный, ровный, без истерик
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
— Если начало raw даже частично, примерно или смыслово дублирует конец previous — считать это повтором и вырезать полностью. Неважно, совпадают ли слова буквально или только по смыслу.
— Полностью удалить совпадающее или частично совпадающее начало.
— Продолжить фрагмент с маленькой буквы, без имени говорящего.
— Текст переписать в нормальную речь.

Если raw звучит как новая реплика (новая интонация, «ну», «а», «так», «ладно», «короче», «слушай», «итак» и т.п.):
— Начать строго с новой строки.
— Строка 1: СПИКЕР:
— Строка 2: уже переписанный, чистый, гладкий текст (новый абзац).
— Никаких сшиваний с previous.
— Никакого повторения previous.

Стиль:
— Плавная, нормальная, человеческая речь.
— Никаких повторов.
— Выправлять синтаксис.
— Разбивать на короткие абзацы.
— Смысл сохранять.

Формат вывода:
— Только новый чанк.
— Никаких HTML.
— Никаких служебных слов.
— Один чистый читабельный фрагмент, готовый к склейке.`

	body := orRequest{
		Model:     "openai/gpt-5.1",
		MaxTokens: 300,
		Messages: []orMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("Previous:\n%s\n\nRaw:\n%s", prev, raw)},
		},
	}

	j, _ := json.Marshal(body)

	log.Printf("[GPT] request prev=%.40q raw=%.40q", prev, raw)

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
		log.Printf("[GPT] http err=%v", err)
		return "", err
	}
	defer resp.Body.Close()

	rawResp, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		log.Printf("[GPT] bad status=%d body=%.200s", resp.StatusCode, rawResp)
		return "", fmt.Errorf("gpt status %d", resp.StatusCode)
	}

	var out orResponse
	if err := json.Unmarshal(rawResp, &out); err != nil {
		log.Printf("[GPT] decode err=%v", err)
		return "", err
	}

	if len(out.Choices) == 0 {
		log.Printf("[GPT] empty choices")
		return "", fmt.Errorf("no choices")
	}

	res := out.Choices[0].Message.Content

	log.Printf("[GPT] ok out=%.80q", res)
	return res, nil
}
