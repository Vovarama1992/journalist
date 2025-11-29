package domain

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

func ResolveYouTube(src string) (string, error) {
	const yt = "/usr/local/bin/yt-dlp"

	// itag=140 — прямой AAC-аудиофайл (стабильно, без DASH/HLS)
	args := []string{"-f", "140", "--no-playlist", "-g", src}

	log.Printf("[YT] cmd: yt-dlp %v", args)

	out, err := exec.Command(yt, args...).CombinedOutput()
	res := strings.TrimSpace(string(out))

	log.Printf("[YT] raw out: %q", res)

	if err != nil {
		return "", fmt.Errorf("yt-dlp itag=140 failed: %w, out=%s", err, res)
	}
	if res == "" || !strings.HasPrefix(res, "http") {
		return "", fmt.Errorf("invalid yt-dlp output: %q", res)
	}
	if !strings.Contains(res, "videoplayback") {
		return "", fmt.Errorf("not a direct audio URL: %q", res)
	}

	log.Printf("[YT] RESOLVED itag=140: %.120s", res)
	return res, nil
}
