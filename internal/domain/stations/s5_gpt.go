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
	// ---- INPUT LOG ----
	pPrev := trim(prev, maxPrevPreview)
	pRaw := trim(raw, maxRawPreview)

	log.Printf("[S5] run prev=%q raw=%q", pPrev, pRaw)

	// ---- CALL GPT ----
	out, err := s.gpt.ProcessChunk(ctx, prev, raw)
	if err != nil {
		log.Printf("[S5] err=%v", err)
		return "", err
	}

	// ---- OUTPUT LOG ----
	pOut := trim(out, maxOutPreview)
	log.Printf("[S5] ok gpt=%q", pOut)

	return out, nil
}
