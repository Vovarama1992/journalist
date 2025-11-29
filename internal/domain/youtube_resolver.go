package domain

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// ResolveYouTube — максимально простой обёртка над тем,
// что у тебя уже проверено в контейнере:
//
//	yt-dlp -g <url>
func ResolveYouTube(src string) (string, error) {
	const ytPath = "/usr/local/bin/yt-dlp"

	// ЛОГИРУЕМ команду, чтобы потом видеть 1-в-1
	log.Printf("[media] yt-dlp cmd: %s -g %s", ytPath, src)

	cmd := exec.Command(ytPath, "-g", src)
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))

	if err != nil {
		// Отдаём ВСЁ, чтобы по логам дальше не гадать
		return "", fmt.Errorf("yt-dlp -g failed: %w, output=%s", err, outStr)
	}

	if outStr == "" {
		return "", fmt.Errorf("yt-dlp -g returned empty output")
	}

	lines := strings.Split(outStr, "\n")
	last := strings.TrimSpace(lines[len(lines)-1])

	if !strings.HasPrefix(last, "http") {
		return "", fmt.Errorf("yt-dlp -g returned non-http line: %q (full=%q)", last, outStr)
	}

	log.Printf("[media] ResolveYouTube OK: %.80s…", last)
	return last, nil
}
