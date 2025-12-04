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
	Model    string      `json:"model"`
	Messages []orMessage `json:"messages"`
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
	systemPrompt := `Тебе даются:
1) previous — уже обработанный текст предыдущих чанков
2) raw — сырой текст нового чанка

Задача:
— Аккуратно продолжить текст.
— Сгладить стык между previous и raw.
— Обработать только raw.
— НЕ возвращать previous.
— Выдать один читаемый фрагмент.

Правила:
1) Если новый raw — это продолжение предыдущей реплики того же говорящего,
   тогда:
   — НЕ начинать с новой строки
   — НЕ добавлять нового спикера
   — начинать с маленькой буквы (если по смыслу это продолжение)
   — просто продолжить текст как единый поток.

2) Если начинается НОВАЯ реплика:
   — сделать новую строку
   — добавить имя говорящего КАПСЛОКОМ + двоеточие
     например: **СПИКЕР:**
   — затем текст реплики.

3) Если имя неизвестно — всегда использовать **СПИКЕР:**.

4) Разбивать текст на короткие смысловые абзацы.

5) Убирать повторы, обрывы, дубли слов, незаконченные куски.

Формат вывода:
— Только обработанный новый чанк.
— Структура в стиле живого диалога:
  СПИКЕР:
  текст…

  СПИКЕР:
  текст…

— Никаких HTML, никаких тэгов.`

	body := orRequest{
		Model: "openai/gpt-5.1",
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
