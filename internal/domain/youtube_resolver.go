package domain

import (
	"os/exec"
	"strings"
)

// ResolveYouTube получает прямой audio-URL через yt-dlp -g.
// ВСЁ. Никаких fallback, никак не мудрим.
// yt-dlp сам давал стабильный stream.
// Это была самая рабочая версия.
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

	// Берём последнюю строку — там всегда прямой URL
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return lines[len(lines)-1], nil
}
