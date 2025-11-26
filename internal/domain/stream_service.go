package domain

import (
	"os/exec"
	"strings"

	"github.com/Vovarama1992/go-utils/logger"
)

type StreamService struct {
	log *logger.ZapLogger
}

type StreamInfo struct {
	Format string `json:"format"`
	Raw    string `json:"raw"`
	Video  bool   `json:"video"`
	Audio  bool   `json:"audio"`
}

func NewStreamService(log *logger.ZapLogger) *StreamService {
	return &StreamService{log: log}
}

func (s *StreamService) Probe(url string) (*StreamInfo, error) {
	s.log.Log(logger.LogEntry{
		Level:   "info",
		Message: "probe: start",
		Fields:  map[string]any{"url": url},
	})

	resolved := url
	isYT := strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be")

	if isYT {
		u, err := ResolveYouTube(url)
		if err != nil {
			return &StreamInfo{Format: "youtube", Raw: err.Error()}, nil
		}
		resolved = u
	}

	cmd := exec.Command(
		"ffprobe",
		"-v", "quiet",
		"-headers", "User-Agent: Mozilla/5.0",
		"-show_format",
		"-show_streams",
		"-print_format", "json",
		resolved,
	)

	out, err := cmd.CombinedOutput()
	raw := string(out)

	return &StreamInfo{
		Format: resolved,
		Raw:    raw,
		Video:  strings.Contains(raw, `"codec_type":"video"`),
		Audio:  strings.Contains(raw, `"codec_type":"audio"`),
	}, err
}
