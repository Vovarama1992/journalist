package stations

import (
	"context"
	"log"

	"github.com/Vovarama1992/journalist/internal/ports"
)

const maxS4Preview = 180

type S4WAVtoText struct {
	stt ports.STTService
}

func NewS4WAVtoText(stt ports.STTService) *S4WAVtoText {
	return &S4WAVtoText{stt: stt}
}

func (s *S4WAVtoText) Run(ctx context.Context, wav []byte) (string, error) {
	log.Printf("[S4] run wav=%d bytes", len(wav))

	txt, _, err := s.stt.Recognize(ctx, wav)
	if err != nil {
		log.Printf("[S4] err=%v", err)
		return "", err
	}

	// режем длинный текст
	preview := txt
	if len(preview) > maxS4Preview {
		preview = preview[:maxS4Preview] + "…"
	}

	log.Printf("[S4] ok text=%q", preview)
	return txt, nil
}
