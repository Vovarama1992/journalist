package stations

import (
	"context"
	"log"

	"github.com/Vovarama1992/journalist/internal/ports"
)

const (
	maxPrevPreview = 120
	maxRawPreview  = 120
	maxOutPreview  = 180
)

type S5GPT struct {
	gpt ports.GPTService
}

func NewS5GPT(gpt ports.GPTService) *S5GPT {
	return &S5GPT{gpt: gpt}
}

func trim(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func (s *S5GPT) Run(ctx context.Context, prev, raw string) (string, error) {
	// ---- INPUT LOGS (то, что GPT получает) ----
	log.Printf("[S5][PREV] %q", trim(prev, maxPrevPreview))
	log.Printf("[S5][RAW ] %q", trim(raw, maxRawPreview))

	// ---- CALL GPT ----
	out, err := s.gpt.ProcessChunk(ctx, prev, raw)
	if err != nil {
		log.Printf("[S5][ERR ] %v", err)
		return "", err
	}

	// ---- OUTPUT LOG (что GPT вернул — human readable) ----
	log.Printf("[S5][HUMAN] %q", trim(out, maxOutPreview))

	return out, nil
}
