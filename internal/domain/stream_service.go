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

func NewStreamService(log *logger.ZapLogger) *StreamService {
	return &StreamService{log: log}
}

type StreamInfo struct {
	Format   string `json:"format"`
	Raw      string `json:"raw"`
	HasVideo bool   `json:"video"`
	HasAudio bool   `json:"audio"`
}

func (s *StreamService) Probe(url string) (*StreamInfo, error) {

	s.log.Log(logger.LogEntry{
		Level:   "info",
		Message: "start probe",
		Fields:  map[string]any{"url": url},
	})

	resolved := url

	// --- YouTube ---
	if strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be") {
		s.log.Log(logger.LogEntry{
			Level:   "info",
			Message: "youtube detected, running yt-dlp",
		})

		out, err := exec.Command("yt-dlp", "--dump-json", url).Output()
		if err != nil {
			s.log.Log(logger.LogEntry{
				Level:   "error",
				Message: "yt-dlp failed",
				Fields:  map[string]any{"err": err.Error()},
			})

			return &StreamInfo{
				Format: "youtube",
				Raw:    string(out),
			}, nil
		}

		var j map[string]any
		json.Unmarshal(out, &j)

		rf, ok := j["requested_formats"].([]any)
		if ok && len(rf) > 0 {
			f0 := rf[0].(map[string]any)
			if manifest, ok := f0["manifest_url"].(string); ok {
				resolved = manifest

				s.log.Log(logger.LogEntry{
					Level:   "info",
					Message: "youtube manifest resolved",
					Fields:  map[string]any{"resolved": resolved},
				})
			}
		}
	}

	// --- ffprobe ---
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_format",
		"-show_streams",
		"-print_format", "json",
		resolved,
	)

	out, err := cmd.CombinedOutput()
	raw := string(out)

	s.log.Log(logger.LogEntry{
		Level:   "info",
		Message: "ffprobe result",
		Fields:  map[string]any{"raw": raw[:min(300, len(raw))]},
	})

	return &StreamInfo{
		Format:   resolved,
		Raw:      raw,
		HasVideo: strings.Contains(raw, `"codec_type":"video"`),
		HasAudio: strings.Contains(raw, `"codec_type":"audio"`),
	}, err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
