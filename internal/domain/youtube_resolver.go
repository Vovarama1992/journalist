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

	// 1) получаем список форматов
	log.Printf("[yt] listing formats: %s -F %s", yt, src)
	listCmd := exec.Command(yt, "-F", src)
	listOut, err := listCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("yt-dlp -F failed: %w, out=%s", err, string(listOut))
	}

	rows := strings.Split(string(listOut), "\n")
	var chosen string

	// 2) выбираем формат — приоритет: 96, 95, 94, 93...
	preferred := []string{"96", "95", "94", "93", "92", "91"}

	for _, p := range preferred {
		for _, row := range rows {
			if strings.HasPrefix(strings.TrimSpace(row), p+" ") {
				chosen = p
				break
			}
		}
		if chosen != "" {
			break
		}
	}

	if chosen == "" {
		return "", fmt.Errorf("no suitable audio/video format found in yt-dlp -F output")
	}

	log.Printf("[yt] chosen format: %s", chosen)

	// 3) получаем прямой URL
	log.Printf("[yt] resolve: %s -f %s -g %s", yt, chosen, src)
	urlCmd := exec.Command(yt, "-f", chosen, "-g", src)
	out, err := urlCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("yt-dlp -g failed: %w, out=%s", err, string(out))
	}

	final := strings.TrimSpace(string(out))
	if !strings.HasPrefix(final, "http") {
		return "", fmt.Errorf("yt-dlp returned non-http: %q", final)
	}

	log.Printf("[yt] resolved direct media URL: %.80s…", final)
	return final, nil
}
