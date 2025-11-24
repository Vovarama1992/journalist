package delivery

import (
	"encoding/json"
	"net/http"

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

	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"msg":    "вижу отличный URL",
	})
}
