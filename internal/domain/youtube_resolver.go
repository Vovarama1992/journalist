package domain

import (
	"os/exec"
	"strings"
)

// ResolveYouTube — максимально тупой, но живучий резолвер.
// 1) yt-dlp -g
// 2) fallback: yt-dlp --get-url
// 3) fallback: вернуть исходный URL (ffmpeg сам выберет аудио)
func ResolveYouTube(src string) (string, error) {

	// ---------- try #1: direct audio URL ----------
	out, err := exec.Command(
		"yt-dlp",
		"-f", "bestaudio",
		"-g",
		src,
	).CombinedOutput()

	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "http") {
			return lines[len(lines)-1], nil
		}
	}

	// ---------- try #2: HLS / DASH URL ----------
	out2, err2 := exec.Command(
		"yt-dlp",
		"--get-url",
		"-f", "bestaudio",
		src,
	).CombinedOutput()

	if err2 == nil {
		lines := strings.Split(strings.TrimSpace(string(out2)), "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "http") {
			return lines[len(lines)-1], nil
		}
	}

	// ---------- fallback #3: give original YouTube URL ----------
	// ffmpeg обычно прекрасно парсит сам
	return src, nil
}
