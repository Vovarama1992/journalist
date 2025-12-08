package stations

import (
	"context"

	"github.com/Vovarama1992/journalist/internal/ports"
)

type S5GPT struct {
	gpt ports.GPTService
}

func NewS5GPT(gpt ports.GPTService) *S5GPT {
	return &S5GPT{gpt: gpt}
}

func (s *S5GPT) Run(ctx context.Context, prev, raw string) (string, error) {

	println("[S5] start")

	out, err := s.gpt.ProcessChunk(ctx, prev, raw)
	if err != nil {
		println("[S5] fail")
		return "", err
	}

	println("[S5] ok")
	return out, nil
}
