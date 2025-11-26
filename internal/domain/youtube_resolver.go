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

func (r *YoutubeResolver) Resolve(url string) ([]string, error) {
	cmd := exec.Command("yt-dlp", "-g", url)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")

	return lines, nil
}
