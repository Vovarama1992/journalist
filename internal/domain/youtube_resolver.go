package domain

import (
	"fmt"
	"os/exec"
	"strings"
)

// ResolveYouTube — самый простой и стабильный рабочий резолвер.
// Используем ПОЛНЫЙ путь к yt-dlp, т.к. Go-процесс внутри контейнера
// НЕ видит /usr/local/bin без явного указания.
func ResolveYouTube(url string) (string, error) {

	yt := "/usr/local/bin/yt-dlp"

	cmd := exec.Command(
		yt,
		"-f", "bestaudio",
		"-g",
		url,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w, output=%s", err, string(out))
	}

	// Последняя строка = прямой media URL
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return "", fmt.Errorf("no output from yt-dlp")
	}

	return lines[len(lines)-1], nil
}
