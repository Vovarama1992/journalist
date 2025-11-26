package domain

import (
	"encoding/json"
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

type ProbeResult struct {
	Format  string      `json:"format"`
	Streams interface{} `json:"streams"`
	Raw     string      `json:"raw"`
	Video   bool        `json:"video"`
	Audio   bool        `json:"audio"`
}

func (s *StreamService) Probe(url string) (*ProbeResult, error) {

	// YouTube?
	if strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be") {
		resolved, err := s.youtube.Resolve(url)
		if err != nil {
			return &ProbeResult{
				Format: "youtube",
				Raw:    err.Error(),
			}, nil
		}
		if resolved != "" {
			url = resolved
		}
	}

	// ffprobe
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_format",
		"-show_streams",
		"-print_format", "json",
		url,
	)

	out, err := cmd.CombinedOutput()
	raw := string(out)

	// Парсим структуру
	var parsed map[string]interface{}
	_ = json.Unmarshal(out, &parsed)

	// flags
	video := strings.Contains(raw, `"codec_type":"video"`)
	audio := strings.Contains(raw, `"codec_type":"audio"`)

	format := "unknown"
	if _, ok := parsed["format"]; ok {
		format = "detected"
	}

	return &ProbeResult{
		Format:  format,
		Streams: parsed["streams"],
		Raw:     raw,
		Video:   video,
		Audio:   audio,
	}, err
}
