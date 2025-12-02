package stations

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

const maxPreview = 180 // максимум символов, чтобы не засрать лог

type S1ResolveURL struct{}

func NewS1ResolveURL() *S1ResolveURL { return &S1ResolveURL{} }

func (s *S1ResolveURL) Run(ctx context.Context, pageURL string) (string, error) {
	log.Printf("[S1] run pageURL=%s", pageURL)

	out, err := exec.CommandContext(
		ctx,
		"yt-dlp", "--no-playlist", "-g", pageURL,
	).CombinedOutput()

	raw := strings.TrimSpace(string(out))
	if len(raw) > maxPreview {
		raw = raw[:maxPreview] + "…"
	}

	if err != nil {
		log.Printf("[S1] yt-dlp err=%v", err)
	}

	if raw == "" {
		log.Printf("[S1] empty output")
		return "", fmt.Errorf("empty yt-dlp output")
	}

	parts := strings.Split(raw, "\n")
	last := strings.TrimSpace(parts[len(parts)-1])

	if !strings.HasPrefix(last, "http") {
		log.Printf("[S1] invalid audioURL=%q", last)
		return "", fmt.Errorf("invalid audio url")
	}

	log.Printf("[S1] ok audioURL=%s", last)
	return last, nil
}
