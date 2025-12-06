package stations

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

type S1ResolveURL struct{}

func NewS1ResolveURL() *S1ResolveURL { return &S1ResolveURL{} }

func (s *S1ResolveURL) Run(ctx context.Context, pageURL string) (string, error) {
	log.Printf("[S1] run pageURL=%s", pageURL)

	// больше не используем -f bestaudio — не работает на live
	cmd := exec.CommandContext(ctx,
		"yt-dlp", "--no-playlist", "-g", pageURL,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[S1] yt-dlp err=%v", err)
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return "", fmt.Errorf("empty yt-dlp output")
	}

	lines := strings.Split(raw, "\n")

	var audioURL string
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "http") {
			audioURL = ln
			break
		}
	}

	if audioURL == "" {
		trim := raw
		if len(trim) > 200 {
			trim = trim[:200] + "..."
		}
		log.Printf("[S1] invalid audioURL=%q", trim)
		return "", fmt.Errorf("invalid audio url")
	}

	log.Printf("[S1] ok url=%s", audioURL)
	return audioURL, nil
}
