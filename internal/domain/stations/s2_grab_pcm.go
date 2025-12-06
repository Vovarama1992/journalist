package stations

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
)

const maxS2ErrPreview = 180

type S2GrabPCM struct{}

func NewS2GrabPCM() *S2GrabPCM {
	return &S2GrabPCM{}
}

func (s *S2GrabPCM) Run(ctx context.Context, audioURL string) ([]byte, error) {
	log.Printf("[S2] run audioURL=%s", audioURL)

	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-i", audioURL,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-t", "15",
		"-f", "s16le",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("[S2] stdout pipe: %w", err)
	}

	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("[S2] ffmpeg start: %w", err)
	}

	// читаем stderr коротко (как раньше)
	go func() {
		b, _ := io.ReadAll(stderr)
		if len(b) > 0 {
			msg := string(b)
			if len(msg) > maxS2ErrPreview {
				msg = msg[:maxS2ErrPreview] + "…"
			}
			log.Printf("[S2] stderr: %s", msg)
		}
	}()

	// ----------- FIX: корректное чтение stdout с ctx -----------
	var pcm []byte
	buf := make([]byte, 4096)

readLoop:
	for {
		select {
		case <-ctx.Done():
			// убьёт ffmpeg, stdout закроется, завершаем
			_ = cmd.Process.Kill()
			return nil, ctx.Err()

		default:

		}

		n, err := stdout.Read(buf)
		if n > 0 {
			pcm = append(pcm, buf[:n]...)
		}

		if err != nil {
			if err == io.EOF {
				break readLoop
			}
			return nil, fmt.Errorf("[S2] read pcm: %w", err)
		}
	}
	// ----------------------------------------------------------

	_ = cmd.Wait()

	if len(pcm) == 0 {
		log.Printf("[S2] empty pcm")
		return pcm, nil
	}

	log.Printf("[S2] ok pcm=%d bytes", len(pcm))
	return pcm, nil
}
