package stations

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"time"
)

const maxS2ErrPreview = 180

type S2GrabPCM struct{}

func NewS2GrabPCM() *S2GrabPCM {
	return &S2GrabPCM{}
}

func (s *S2GrabPCM) Run(ctx context.Context, audioURL string) ([]byte, error) {
	start := time.Now()
	log.Printf("[S2][START] url=%s", audioURL)

	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-loglevel", "error",
		"-i", audioURL,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-t", "20", // <<< УВЕЛИЧЕННОЕ ОКНО
		"-f", "s16le",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("[S2] stdout pipe: %w", err)
	}

	stderr, _ := cmd.StderrPipe()
	go func() {
		b, _ := io.ReadAll(stderr)
		if len(b) > 0 {
			log.Printf("[S2][STDERR] %s", string(b))
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("[S2] ffmpeg start: %w", err)
	}

	var pcm []byte
	buf := make([]byte, 4096)

	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			pcm = append(pcm, buf[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			_ = cmd.Process.Kill()
			return nil, fmt.Errorf("[S2] read pcm: %w", err)
		}
	}

	_ = cmd.Wait()

	dur := time.Since(start)
	if len(pcm) == 0 {
		log.Printf("[S2][EMPTY] dur=%s", dur)
		return pcm, nil
	}

	log.Printf(
		"[S2][OK] bytes=%d approx_sec=%.1f dur=%s",
		len(pcm),
		float64(len(pcm))/2/16000,
		dur,
	)

	return pcm, nil
}
