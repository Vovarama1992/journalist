package domain

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// ResolveYouTube получает прямой URL (audio-only) для ffmpeg
func ResolveYouTube(url string) (string, error) {
	cmd := exec.Command(
		"yt-dlp",
		"--no-check-certificate",
		"-f", "bestaudio[ext=m4a]/bestaudio*/best",
		"-g",
		url,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[yt-dlp] ERROR: %s", string(out))
		return "", fmt.Errorf("yt-dlp failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return lines[len(lines)-1], nil
}
