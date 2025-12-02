package stations

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
)

// S2GrabPCM — station 2: audioURL -> PCM (s16le, 16kHz, mono)
type S2GrabPCM struct{}

func NewS2GrabPCM() *S2GrabPCM {
	return &S2GrabPCM{}
}

func (s *S2GrabPCM) Run(ctx context.Context, audioURL string) ([]byte, error) {
	log.Printf("[S2 IN] audioURL=%s", audioURL)

	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-i", audioURL,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-t", "15", // <<<<<<<< заменили 5 на 15
		"-f", "s16le",
		"pipe:1",
	)

	log.Printf("[S2 INSIDE] run: ffmpeg -i <audio> -ac 1 -ar 16000 -t 15 -f s16le pipe:1")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("[S2 OUT] stdout pipe: %w", err)
	}
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("[S2 OUT] ffmpeg start: %w", err)
	}

	// логируем stderr целиком
	go func() {
		b, _ := io.ReadAll(stderr)
		if len(b) > 0 {
			log.Printf("[S2 INSIDE FFMPEG STDERR] %s", b)
		}
	}()

	pcm, err := io.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("[S2 OUT] read pcm: %w", err)
	}

	log.Printf("[S2 INSIDE] pcmBytes=%d", len(pcm))
	if len(pcm) > 0 {
		previewN := 32
		if len(pcm) < previewN {
			previewN = len(pcm)
		}
		log.Printf("[S2 INSIDE] pcm[0:%d]=%v", previewN, pcm[:previewN])
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("[S2 INSIDE] ffmpeg exit err=%v", err)
		// всё равно считаем чанк валидным, если байты есть
	}

	log.Printf("[S2 OUT] pcm=%d bytes", len(pcm))
	return pcm, nil
}
