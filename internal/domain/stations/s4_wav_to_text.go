package stations

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Vovarama1992/journalist/internal/ports"
)

// S4WAVtoText — station 4: WAV -> текст (через Yandex STT)
type S4WAVtoText struct {
	stt ports.STTService
}

func NewS4WAVtoText(stt ports.STTService) *S4WAVtoText {
	return &S4WAVtoText{stt: stt}
}

func (s *S4WAVtoText) Run(ctx context.Context, wav []byte) (string, error) {
	log.Printf("[S4 IN] wav=%d", len(wav))

	txt, raw, err := s.stt.Recognize(ctx, wav)

	// полный сырой ответ Яндекса
	if len(raw) > 0 {
		log.Printf("[S4 RAW YANDEX] %s", raw)
	}

	log.Printf("[S4 INSIDE] txt=%q err=%v", txt, err)

	if err != nil {
		return "", fmt.Errorf("[S4 OUT] stt err: %w", err)
	}

	txt = strings.TrimSpace(txt)
	log.Printf("[S4 OUT] txt=%q", txt)
	return txt, nil
}
