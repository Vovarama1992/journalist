package stations

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

type S1ResolveURL struct {
	cookieFile string
}

func NewS1ResolveURL(cookieFile string) *S1ResolveURL {
	return &S1ResolveURL{cookieFile: cookieFile}
}

func (s *S1ResolveURL) Run(ctx context.Context, pageURL string) (string, error) {
	log.Printf("[S1][START] page=%q", pageURL)

	args := []string{
		"--no-playlist",
		"-g",
	}

	// если куки есть → добавляем
	if s.cookieFile != "" {
		args = append(args, "--cookies", s.cookieFile)
	}

	args = append(args, pageURL)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	out, err := cmd.CombinedOutput()

	if err != nil {
		log.Printf("[S1][ERR-exec] %v", err)
	}

	raw := strings.TrimSpace(string(out))
	log.Printf("[S1][RAW] %q", trim(raw, 280))

	if raw == "" {
		log.Printf("[S1][ERR] empty output")
		return "", fmt.Errorf("empty yt-dlp output")
	}

	// ищем первую http-строку
	for _, ln := range strings.Split(raw, "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "http") {
			log.Print("[S1][OK]")
			return ln, nil
		}
	}

	log.Printf("[S1][ERR] parsed url empty")
	return "", fmt.Errorf("parsed url empty")
}
