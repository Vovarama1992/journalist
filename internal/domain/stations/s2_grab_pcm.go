package stations

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

const maxS2ErrPreview = 180

type S2GrabPCM struct{}

func NewS2GrabPCM() *S2GrabPCM {
	return &S2GrabPCM{}
}

func (s *S2GrabPCM) Run(ctx context.Context, audioURL string) ([]byte, error) {

	println("[S2] start")

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
		println("[S2] fail")
		return nil, fmt.Errorf("[S2] stdout pipe: %w", err)
	}

	// ВОЗВРАЩАЕМ как было — ЧИТАЕМ stderr (НО БЕЗ ЛОГОВ)
	stderr, _ := cmd.StderrPipe()
	go func() {
		b, _ := io.ReadAll(stderr)
		_ = b // глотаем, не логируем
	}()

	if err := cmd.Start(); err != nil {
		println("[S2] fail")
		return nil, fmt.Errorf("[S2] ffmpeg start: %w", err)
	}

	var pcm []byte
	buf := make([]byte, 4096)

readLoop:
	for {
		select {
		case <-ctx.Done():
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
			println("[S2] fail")
			return nil, fmt.Errorf("[S2] read pcm: %w", err)
		}
	}

	_ = cmd.Wait()

	if len(pcm) == 0 {
		println("[S2] fail")
		return pcm, nil
	}

	println("[S2] ok")
	return pcm, nil
}
