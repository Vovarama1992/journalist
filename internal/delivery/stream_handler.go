package delivery

import (
	"encoding/json"
	"net/http"

	"github.com/Vovarama1992/go-utils/logger"
	"github.com/Vovarama1992/journalist/internal/domain"
)

type StreamHandler struct {
	log     *logger.ZapLogger
	service *domain.StreamService
}

func NewStreamHandler(
	log *logger.ZapLogger,
	service *domain.StreamService,
) *StreamHandler {
	return &StreamHandler{
		log:     log,
		service: service,
	}
}

func (h *StreamHandler) Start(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	h.log.Log(logger.LogEntry{
		Level:   "info",
		Message: "probe stream: " + url,
	})

	info, _ := h.service.Probe(url)

	_ = json.NewEncoder(w).Encode(info)
}
