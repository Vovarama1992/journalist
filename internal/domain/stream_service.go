package domain

import (
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/Vovarama1992/go-utils/logger"
)

type StreamService struct {
	log *logger.ZapLogger
}

type StreamInfo struct {
	Source      string `json:"source"`
	ResolvedURL string `json:"resolved_url"`
	HasVideo    bool   `json:"has_video"`
	HasAudio    bool   `json:"has_audio"`
	Summary     string `json:"summary"`
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
	source := "generic"

	isYT := strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be")
	if isYT {
		source = "youtube"
		u, err := ResolveYouTube(url)
		if err != nil {
			return &StreamInfo{
				Source:      source,
				ResolvedURL: "",
				HasVideo:    false,
				HasAudio:    false,
				Summary:     "Не удалось получить прямую ссылку",
			}, nil
		}
		resolved = u
	}

	s.log.Log(logger.LogEntry{
		Level:   "info",
		Message: "resolved url",
		Fields:  map[string]any{"url": resolved},
	})

	cmd := exec.Command(
		"ffprobe",
		"-v", "quiet",
		"-headers", "User-Agent: Mozilla/5.0",
		"-headers", "Accept: */*",
		"-headers", "Referer: https://www.youtube.com/",
		"-headers", "Origin: https://www.youtube.com",
		"-show_format",
		"-show_streams",
		"-print_format", "json",
		resolved,
	)

	out, _ := cmd.CombinedOutput()
	raw := string(out)

	s.log.Log(logger.LogEntry{
		Level:   "info",
		Message: "ffprobe raw",
		Fields:  map[string]any{"raw": raw},
	})

	type ffprobeResponse struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
		} `json:"streams"`
	}

	var parsed ffprobeResponse
	_ = json.Unmarshal(out, &parsed)

	hasVideo := false
	hasAudio := false

	for _, st := range parsed.Streams {
		if st.CodecType == "video" {
			hasVideo = true
		}
		if st.CodecType == "audio" {
			hasAudio = true
		}
	}

	summary := "Нет аудио/видео"
	if hasVideo && hasAudio {
		summary = "Есть видео + аудио"
	} else if hasVideo {
		summary = "Есть только видео"
	} else if hasAudio {
		summary = "Есть только аудио"
	}

	return &StreamInfo{
		Source:      source,
		ResolvedURL: resolved,
		HasVideo:    hasVideo,
		HasAudio:    hasAudio,
		Summary:     summary,
	}, nil
}
