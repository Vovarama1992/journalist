package domain

import (
	"os/exec"
	"strings"
)

type StreamService struct {
	youtube *YoutubeResolver
}

func NewStreamService() *StreamService {
	return &StreamService{
		youtube: NewYoutubeResolver(),
	}
}

type StreamInfo struct {
	Format    string
	HasVideo  bool
	HasAudio  bool
	RawOutput string
}

func (s *StreamService) Probe(url string) (*StreamInfo, error) {

	// --- YouTube detection ---
	if strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be") {
		streams, err := s.youtube.Resolve(url)
		if err != nil {
			return &StreamInfo{Format: "youtube", RawOutput: err.Error()}, nil
		}
		// Берём первый поток для ffprobe
		if len(streams) > 0 {
			url = streams[0]
		}
	}

	// --- ffprobe ---
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
		RawOutput: raw,
		HasVideo:  strings.Contains(raw, `"codec_type":"video"`),
		HasAudio:  strings.Contains(raw, `"codec_type":"audio"`),
	}

	if strings.Contains(raw, `"format_name"`) {
		info.Format = "detected"
	}

	return info, err
}
