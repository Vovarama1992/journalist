package stations

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// S1ResolveURL — station 1: page URL -> прямой audio URL через yt-dlp.
type S1ResolveURL struct{}

func NewS1ResolveURL() *S1ResolveURL {
	return &S1ResolveURL{}
}

func (s *S1ResolveURL) Run(ctx context.Context, pageURL string) (string, error) {
	out, err := exec.CommandContext(
		ctx,
		"yt-dlp",
		"--no-playlist",
		"-g",
		pageURL,
	).CombinedOutput()

	raw := strings.TrimSpace(string(out))
	log.Printf("[S1 RESOLVE RAW] %q err=%v", raw, err)

	lines := strings.Split(raw, "\n")
	last := strings.TrimSpace(lines[len(lines)-1])

	if !strings.HasPrefix(last, "http") {
		return "", fmt.Errorf("invalid audio url: %q", last)
	}

	return last, nil
}
