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

	// берём прямой аудиопоток m4a (144k)
	cmd := exec.Command(yt, "-f", "140", "--no-playlist", "-g", src)

	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))

	log.Printf("[yt] raw out: %q", s)

	if err != nil {
		return "", fmt.Errorf("yt-dlp err: %w, out=%s", err, s)
	}

	if !strings.HasPrefix(s, "http") {
		return "", fmt.Errorf("invalid yt-dlp output: %q", s)
	}

	log.Printf("[yt] resolved audio URL: %.100s", s)
	return s, nil
}
