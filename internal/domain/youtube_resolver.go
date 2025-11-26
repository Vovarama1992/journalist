package domain

import (
	"bytes"
	"os/exec"
	"strings"
)

// ResolveYouTube получает прямой URL (обычно audio-only), который ffprobe может открыть
func ResolveYouTube(url string) (string, error) {
	cmd := exec.Command(
		"yt-dlp",
		"-f", "bestaudio",
		"-g",
		url,
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}
