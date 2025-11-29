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
	log.Printf("[PTRP] resolve youtube: %s", src)

	try := func(desc string, args ...string) (string, bool) {
		log.Printf("[PTRP] yt-dlp try: %s → yt-dlp %v", desc, args)

		cmd := exec.Command("yt-dlp", args...)
		out, err := cmd.CombinedOutput()

		log.Printf("[PTRP] yt-dlp out (%s): %s", desc, strings.TrimSpace(string(out)))
		if err != nil {
			log.Printf("[PTRP] yt-dlp err (%s): %v", desc, err)
			return "", false
		}

		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) == 0 {
			log.Printf("[PTRP] yt-dlp (%s): no lines", desc)
			return "", false
		}

		last := lines[len(lines)-1]
		if !strings.HasPrefix(last, "http") {
			log.Printf("[PTRP] yt-dlp (%s): not url: %s", desc, last)
			return "", false
		}

		log.Printf("[PTRP] RESOLVED (%s): %s", desc, last)
		return last, true
	}

	// 1) основной путь — чистый аудио стрим
	if url, ok := try("bestaudio-m4a", "-f", "bestaudio[ext=m4a]/bestaudio", "--no-playlist", "-g", src); ok {
		return url, nil
	}

	// 2) fallback — просто bestaudio
	if url, ok := try("bestaudio", "-f", "bestaudio", "--no-playlist", "-g", src); ok {
		return url, nil
	}

	// 3) последний fallback — как у тебя было (может дать m3u8)
	if url, ok := try("raw", "-g", src); ok {
		return url, nil
	}

	return "", fmt.Errorf("yt-dlp: cannot resolve audio URL for %s", src)
}
