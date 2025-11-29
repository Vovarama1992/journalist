package domain

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ResolveYouTube — максимально тупой, но живучий резолвер.
// 1) yt-dlp -g
// 2) fallback: yt-dlp --get-url
// 3) fallback: вернуть исходный URL (ffmpeg сам выберет аудио)
func ResolveYouTube(src string) (string, error) {

	formats := []string{
		"bestaudio",
		"251", // opus/webm
		"250", // opus/webm lower
		"140", // m4a
	}

	// 3 попытки, каждая — перебор форматов
	for attempt := 1; attempt <= 3; attempt++ {

		for _, fmtID := range formats {

			out, err := exec.Command(
				"yt-dlp",
				"-f", fmtID,
				"-g",
				src,
			).CombinedOutput()

			if err != nil {
				continue
			}

			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) == 0 {
				continue
			}

			u := lines[len(lines)-1]

			// Принцип железобетонности:
			// Не принимаем URL, который начинается с youtube.com
			if strings.Contains(u, "googlevideo.com") ||
				strings.Contains(u, "manifest") ||
				strings.Contains(u, ".m3u8") ||
				strings.Contains(u, ".mp4") {
				return u, nil
			}
		}

		time.Sleep(1 * time.Second)
	}

	// Вообще не удалось — возвращаем ошибку
	return "", fmt.Errorf("yt-dlp failed: no direct media url")
}
