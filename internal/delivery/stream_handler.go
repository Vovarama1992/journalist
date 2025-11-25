package delivery

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"

	"github.com/Vovarama1992/go-utils/logger"
)

type StreamHandler struct {
	log *logger.ZapLogger
}

func NewStreamHandler(log *logger.ZapLogger) *StreamHandler {
	return &StreamHandler{log: log}
}

func (h *StreamHandler) Start(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	h.log.Log(logger.LogEntry{
		Level:   "info",
		Message: "получил URL: " + url,
	})

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

	// --- разбор результата ---
	format := "unknown"
	hasVideo := false
	hasAudio := false

	if strings.Contains(raw, `"codec_type":"video"`) {
		hasVideo = true
	}
	if strings.Contains(raw, `"codec_type":"audio"`) {
		hasAudio = true
	}
	if strings.Contains(raw, `"format_name"`) {
		// выдёргиваем примитивно — этого достаточно
		format = "detected"
	}

	// --- если ffprobe упал — тоже отдаём аккуратно ---
	if err != nil {
		h.log.Log(logger.LogEntry{
			Level:   "warn",
			Message: "ffprobe не смог извлечь потоки",
			Error:   err,
		})

		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":     "ok",
			"format":     format,
			"video":      hasVideo,
			"audio":      hasAudio,
			"note":       "Некоторые источники (например, YouTube-ссылки) не дают прямой поток без спец-обработки — это нормально.",
			"ffprobeRaw": raw,
		})
		return
	}

	// --- успех ---
	h.log.Log(logger.LogEntry{
		Level:   "info",
		Message: "ffprobe: стрим доступен, формат=" + format,
	})

	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":     "ok",
		"format":     format,
		"video":      hasVideo,
		"audio":      hasAudio,
		"ffprobeRaw": raw,
	})
}
