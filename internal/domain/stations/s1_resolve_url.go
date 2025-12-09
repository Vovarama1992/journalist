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

	log.Printf("[S1][START] page=%q", pageURL)

	cmd := exec.CommandContext(ctx,
		"yt-dlp", "--no-playlist", "-g", pageURL,
	)

	out, err := cmd.CombinedOutput()

	// логируем сырой вывод
	log.Printf("[S1][RAW] %q", trim(string(out), 500))

	if err != nil {
		log.Printf("[S1][ERR-exec] %v", err)
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		log.Printf("[S1][ERR] empty output")
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
		log.Printf("[S1][ERR] parsed url empty")
		return "", fmt.Errorf("invalid audio url")
	}

	log.Printf("[S1][OK] url=%q", audioURL)
	return audioURL, nil
}
