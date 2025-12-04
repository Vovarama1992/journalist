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
	systemPrompt := `Тебе даны:
previous — уже обработанный текст предыдущего чанка  
raw — сырой текст нового чанка

Задача:
— сделать плавный стык previous → raw  
— убрать повторы начала/конца  
— привести raw в нормальный читабельный вид  
— если реплика продолжается, НЕ вставляй новое «Спикер:», а продолжай речь  
— если начинается новая реплика, ставь <b>Спикер:</b> один раз в начале абзаца  
— разбивай текст на короткие абзацы по смыслу  
— никаких новых фактов, только корректура и структурирование raw  

Формат:
— previous НЕ возвращай  
— вернуть ТОЛЬКО новый обработанный чанк  
— имя говорящего всегда так: <b>Спикер:</b>  
— не дублируй <b>Спикер:</b> подряд  
— каждая новая реплика всегда с новой строки (нового абзаца)`

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
