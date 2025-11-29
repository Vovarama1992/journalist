package domain

import (
	"os/exec"
	"strings"
)

// ResolveYouTube получает прямой URL (audio-only) для ffmpeg
func ResolveYouTube(url string) (string, error) {
	cmd := exec.Command(
		"yt-dlp",
		"-f", "bestaudio",
		"-g",
		url,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return lines[len(lines)-1], nil
}
