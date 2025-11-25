package delivery

import (
	"encoding/json"
	"net/http"
	"os/exec"

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

	// --- лёгкая проверка стрима через ffprobe ---
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_format",
		"-show_streams",
		"-print_format", "json",
		url,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		h.log.Log(logger.LogEntry{
			Level:   "error",
			Message: "ffprobe error",
			Error:   err,
		})

		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "error",
			"msg":    "не удалось подключиться к стриму",
			"raw":    string(out),
		})
		return
	}

	h.log.Log(logger.LogEntry{
		Level:   "info",
		Message: "ffprobe: стрим доступен",
	})

	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"msg":    "вижу отличный URL",
		"probe":  json.RawMessage(out),
	})
}
