package stations

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type S1ResolveURL struct{}

func NewS1ResolveURL() *S1ResolveURL { return &S1ResolveURL{} }

func (s *S1ResolveURL) Run(ctx context.Context, pageURL string) (string, error) {

	println("[S1] start")

	cmd := exec.CommandContext(ctx,
		"yt-dlp", "--no-playlist", "-g", pageURL,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		println("[S1] fail")
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		println("[S1] fail")
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
		println("[S1] fail")
		return "", fmt.Errorf("invalid audio url")
	}

	println("[S1] ok")
	return audioURL, nil
}
