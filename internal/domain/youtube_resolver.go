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
	const yt = "/usr/local/bin/yt-dlp"

	cmd := exec.Command(yt, "-f", "140", "--no-playlist", "-g", src)
	out, err := cmd.CombinedOutput()
	raw := strings.TrimSpace(string(out))

	log.Printf("[yt] raw out: %q", raw)

	if err != nil {
		// даже если ошибка — пробуем достать URL из вывода
		log.Printf("[yt] err: %v", err)
	}

	lines := strings.Split(raw, "\n")
	last := strings.TrimSpace(lines[len(lines)-1])

	if !strings.HasPrefix(last, "http") {
		return "", fmt.Errorf("yt-dlp: didn't find http url, last=%q", last)
	}

	log.Printf("[yt] resolved audio URL: %.120s", last)
	return last, nil
}
