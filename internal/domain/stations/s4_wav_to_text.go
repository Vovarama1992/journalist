package stations

import (
	"context"

	"github.com/Vovarama1992/journalist/internal/ports"
)

type S4WAVtoText struct {
	stt ports.STTService
}

func NewS4WAVtoText(stt ports.STTService) *S4WAVtoText {
	return &S4WAVtoText{stt: stt}
}

func (s *S4WAVtoText) Run(ctx context.Context, wav []byte) (string, error) {

	println("[S4] start")

	txt, _, err := s.stt.Recognize(ctx, wav)
	if err != nil {
		println("[S4] fail")
		return "", err
	}

	println("[S4] ok")
	return txt, nil
}
