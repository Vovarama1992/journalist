package domain

import (
	"os/exec"
	"strings"
)

type StreamInfo struct {
	Format    string
	HasVideo  bool
	HasAudio  bool
	RawOutput string
}

type StreamService struct{}

func NewStreamService() *StreamService {
	return &StreamService{}
}

func (s *StreamService) Probe(url string) (*StreamInfo, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_format",
		"-show_streams",
		"-print_format", "json",
		url,
	)

	out, err := cmd.CombinedOutput()
	raw := string(out)

	info := &StreamInfo{
		Format:    "unknown",
		HasVideo:  strings.Contains(raw, `"codec_type":"video"`),
		HasAudio:  strings.Contains(raw, `"codec_type":"audio"`),
		RawOutput: raw,
	}

	if strings.Contains(raw, `"format_name"`) {
		info.Format = "detected"
	}

	return info, err
}
