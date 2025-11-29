package domain

import (
	"fmt"
	"os/exec"
	"strings"
)

func ResolveYouTube(url string) (string, error) {
	yt := "/usr/local/bin/yt-dlp"

	// 1) Получаем ВСЕ форматы
	out, err := exec.Command(yt, "-F", url).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("formats error: %w, %s", err, out)
	}

	lines := strings.Split(string(out), "\n")

	// Ищем подходящий формат (audio или m3u8)
	var chosenFormat string
	for _, line := range lines {
		// audio
		if strings.Contains(line, "audio only") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				chosenFormat = fields[0]
				break
			}
		}
	}

	// если audio нет — берём HLS
	if chosenFormat == "" {
		for _, line := range lines {
			if strings.Contains(line, "m3u8") {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					chosenFormat = fields[0]
					break
				}
			}
		}
	}

	// если вообще ничего — жёстко берём первый формат
	if chosenFormat == "" {
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) > 0 && fields[0][0] >= '0' && fields[0][0] <= '9' {
				chosenFormat = fields[0]
				break
			}
		}
	}

	if chosenFormat == "" {
		return "", fmt.Errorf("cannot determine format for the stream")
	}

	// 2) теперь получаем прямой URL
	out2, err := exec.Command(yt, "-f", chosenFormat, "-g", url).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get-url error: %w, %s", err, out2)
	}

	u := strings.TrimSpace(string(out2))
	if !strings.HasPrefix(u, "http") {
		return "", fmt.Errorf("bad URL from yt-dlp: %s", u)
	}

	return u, nil
}
