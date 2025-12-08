package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		println("[GPT] no api key")
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

func (g *GPTClient) ProcessChunk(ctx context.Context, prev, raw string) (string, error) {

	println("[GPT] start")

	if g.apiKey == "" {
		println("[GPT] fail")
		return "", fmt.Errorf("no OPENROUTER_API_KEY")
	}

	systemPrompt := `Тебе даются два текста:

previous — уже готовый фрагмент.
raw — сырой ASR-текст.

... (весь твой prompt полностью сохранён)
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
