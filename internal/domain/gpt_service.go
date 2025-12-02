package domain

import (
	"context"

	"github.com/Vovarama1992/journalist/internal/ports"
)

type gptService struct {
	client interface {
		Generate(ctx context.Context, previous, nextRaw string) (string, error)
	}
}

func NewGPTService(client interface {
	Generate(ctx context.Context, previous, nextRaw string) (string, error)
}) ports.GPTService {
	return &gptService{client: client}
}

func (s *gptService) ProcessChunk(
	ctx context.Context,
	lastChunk string,
	newChunk string,
) (string, error) {
	return s.client.Generate(ctx, lastChunk, newChunk)
}
