package stations

import (
	"context"
	"log"

	"github.com/Vovarama1992/journalist/internal/ports"
)

func trim(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

type S5GPT struct {
	gpt ports.GPTService
}

func NewS5GPT(gpt ports.GPTService) *S5GPT {
	return &S5GPT{gpt: gpt}
}

func (s *S5GPT) Run(ctx context.Context, prev, raw string) (string, error) {
	log.Printf("[S5][IN-prev] %q", trim(prev, 180))
	log.Printf("[S5][IN-raw ] %q", trim(raw, 180))

	out, err := s.gpt.ProcessChunk(ctx, prev, raw)
	if err != nil {
		log.Printf("[S5][ERR] %v", err)
		return "", err
	}

	log.Printf("[S5][HUMN] %q", trim(out, 220))
	return out, nil
}
