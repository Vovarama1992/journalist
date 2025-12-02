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
	"strings"
	"time"
)

type GPTClient struct {
	client *http.Client
	apiKey string
	model  string
}

func NewGPTClient() *GPTClient {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		log.Println("[GPT][ERROR] OPENROUTER_API_KEY not set")
		panic("OPENROUTER_API_KEY not set")
	}

	return &GPTClient{
		client: &http.Client{
			Timeout: 25 * time.Second,
		},
		apiKey: key,
		model:  "openai/gpt-5.1",
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
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *GPTClient) Generate(
	ctx context.Context,
	previous string,
	nextRaw string,
) (string, error) {

	systemPrompt := "Ты обработчик текстовых транскриптов. Ты всегда получаешь два текста: Previous и Next. Previous уже обработан, его менять не нужно. Next — сырой фрагмент речи. Твоя задача — сделать Next читаемым и естественно стыкующимся с Previous. Возвращай только обработанный Next."

	userPrompt := buildPrompt(previous, nextRaw)

	reqBody := orRequest{
		Model: c.model,
		Messages: []orMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://openrouter.ai/api/v1/chat/completions",
		bytes.NewBuffer(b),
	)
	if err != nil {
		log.Printf("[GPT][ERROR] build request: %v", err)
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("HTTP-Referer", "https://journalist.local")
	req.Header.Set("X-Title", "journalist")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("[GPT][ERROR] http: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)

		var er struct {
			Error interface{} `json:"error"`
		}
		_ = json.Unmarshal(raw, &er)

		switch v := er.Error.(type) {
		case string:
			if v != "" {
				log.Printf("[GPT][ERROR] %s", v)
				return "", fmt.Errorf(v)
			}
		case map[string]interface{}:
			if msg, ok := v["message"].(string); ok && msg != "" {
				log.Printf("[GPT][ERROR] %s", msg)
				return "", fmt.Errorf(msg)
			}
		}

		log.Printf("[GPT][ERROR] raw=%s", string(raw))
		return "", fmt.Errorf(string(raw))
	}

	var parsed orResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		log.Printf("[GPT][ERROR] decode: %v", err)
		return "", err
	}

	if len(parsed.Choices) == 0 {
		log.Printf("[GPT][ERROR] empty choices")
		return "", fmt.Errorf("empty choices")
	}

	return parsed.Choices[0].Message.Content, nil
}

func buildPrompt(previous, next string) string {
	return fmt.Sprintf(
		"Previous:\n%s\n\nNext:\n%s",
		strings.TrimSpace(previous),
		strings.TrimSpace(next),
	)
}
