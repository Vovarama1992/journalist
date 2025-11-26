package domain

import (
	"bytes"
	"os/exec"
	"strings"
)

type YoutubeResolver struct{}

func NewYoutubeResolver() *YoutubeResolver {
	return &YoutubeResolver{}
}

func (r *YoutubeResolver) Resolve(url string) (string, error) {
	cmd := exec.Command("yt-dlp",
		"-g",
		"--no-warnings",
		"--youtube-skip-dash-manifest",
		url,
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 {
		return "", nil
	}

	// первая строка — лучший видео-URL
	return lines[0], nil
}
