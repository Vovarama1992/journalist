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

	log.Printf("[yt] cmd: %s -f ba --no-playlist -g %s", yt, src)

	cmd := exec.Command(yt, "-f", "ba", "--no-playlist", "-g", src)
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))

	log.Printf("[yt] raw out: %q", s)
	if err != nil {
		log.Printf("[yt] err: %v", err)
		return "", fmt.Errorf("yt-dlp ba failed: %w, out=%s", err, s)
	}
	if s == "" || !strings.HasPrefix(s, "http") {
		log.Printf("[yt] bad output: %q", s)
		return "", fmt.Errorf("yt-dlp returned invalid url: %q", s)
	}

	log.Printf("[yt] resolved audio URL: %.80s…", s)
	return s, nil
}
