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
	systemPrompt := `Тебе даются:

previous — уже обработанный текст предыдущих чанков

raw — сырой текст нового чанка

Задача:
— Аккуратно продолжить текст.
— Сгладить стык между previous и raw.
— Обработать только raw.
— НЕ возвращать previous.
— Выдать один читаемый новый фрагмент.
— Исправлять сырой распознанный текст: убирать кривые фразы, приводить речь к внятности.
— ВСЕГДА думать о том, как новый чанк будет звучать после приклейки к предыдущему.

Правила:

Если raw — это продолжение предыдущей реплики того же говорящего:
— сначала ПРОВЕРЬ перекрытие: raw часто начинается с повторения хвоста previous;
— найди максимальное совпадение начала raw с концом previous;
— удали это совпадение полностью;
— НЕ начинать с новой строки;
— НЕ добавлять СПИКЕР;
— начинать с маленькой буквы (если по смыслу это продолжение).

Если начинается НОВАЯ реплика:
— новая строка
— имя говорящего КАПСЛОКОМ + двоеточие
— если имя неизвестно — использовать СПИКЕР:.

Определение новой реплики:
— междометие / обращение / новая интонация → новая реплика
— новая самостоятельная мысль → новая реплика
— если raw дублирует хвост previous → это НЕ новая реплика, сначала удалить дублирование.

Стиль:
— убирать шумы, оговорки, дубли, обрывы
— приводить речь к естественной форме
— сохранять смысл
— разбивать на короткие абзацы

Удаление мусора:
— если в raw есть рекламные, технические или служебные вставки («ставьте лайк», «подписывайтесь», «не пропустите новое видео», «это был такой-то канал»), вырезать их полностью.

Требования к выводу:
— Вернуть только обработанный новый чанк.
— Формат диалога:
СПИКЕР:
текст…

СПИКЕР:
текст…

— Никаких html-тегов.`

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
