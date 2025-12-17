package stations

import (
	"context"
	"log"
	"time"

	"github.com/Vovarama1992/journalist/internal/ports"
)

type S4WAVtoText struct {
	stt ports.STTService
}

func NewS4WAVtoText(stt ports.STTService) *S4WAVtoText {
	return &S4WAVtoText{stt: stt}
}

func (s *S4WAVtoText) Run(ctx context.Context, wav []byte) (string, error) {
	log.Printf("[S4][START] wav_bytes=%d", len(wav))

	txt, _, err := s.stt.Recognize(ctx, wav)
	if err == nil {
		log.Printf("[S4][OK]")
		return txt, nil
	}

	log.Printf("[S4][ERR][FIRST] err=%v", err)

	// === RETRY 1 раз с новым контекстом ===
	retryCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	txt, _, err2 := s.stt.Recognize(retryCtx, wav)
	if err2 == nil {
		log.Printf("[S4][OK][RETRY]")
		return txt, nil
	}

	log.Printf("[S4][ERR][RETRY] err=%v", err2)
	return "", err2
}
