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
	const ytPath = "/usr/local/bin/yt-dlp"

	type variant struct {
		name string
		args []string
	}

	variants := []variant{
		{
			name: "bestaudio-m4a",
			args: []string{"-f", "bestaudio[ext=m4a]/bestaudio", "--no-playlist", "-g", src},
		},
		{
			name: "bestaudio",
			args: []string{"-f", "bestaudio", "--no-playlist", "-g", src},
		},
		{
			name: "raw",
			args: []string{"-g", src},
		},
	}

	for _, v := range variants {
		log.Printf("[PTRP] yt-dlp try: %s → yt-dlp %v", v.name, append([]string{}, v.args...))

		cmd := exec.Command(ytPath, v.args...)
		out, err := cmd.CombinedOutput()
		outStr := strings.TrimSpace(string(out))

		if err != nil {
			log.Printf("[PTRP] yt-dlp err (%s): %v", v.name, err)
			log.Printf("[PTRP] yt-dlp out (%s): %s", v.name, outStr)
			continue
		}

		if outStr == "" {
			log.Printf("[PTRP] yt-dlp empty output (%s)", v.name)
			continue
		}

		lines := strings.Split(outStr, "\n")
		last := strings.TrimSpace(lines[len(lines)-1])
		if !strings.HasPrefix(last, "http") {
			log.Printf("[PTRP] yt-dlp non-http line (%s): %q (full=%q)", v.name, last, outStr)
			continue
		}

		log.Printf("[PTRP] RESOLVED (%s): %.80s…", v.name, last)
		return last, nil
	}

	return "", fmt.Errorf("yt-dlp: all variants failed for %s", src)
}
