package delivery

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Vovarama1992/go-utils/logger"
	"github.com/Vovarama1992/journalist/internal/ports"
	"github.com/go-chi/chi/v5"
)

type MediaHandler struct {
	media ports.MediaRepository
	log   *logger.ZapLogger
}

func NewMediaHandler(media ports.MediaRepository, log *logger.ZapLogger) *MediaHandler {
	return &MediaHandler{
		media: media,
		log:   log,
	}
}

// GET /api/media-history/{id}
func (h *MediaHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	text, err := h.media.GetMediaHistory(r.Context(), id)
	if err != nil {
		http.Error(w, "failed get history: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.log.Log(logger.LogEntry{
		Level:   "info",
		Message: "media history fetched",
		Fields: map[string]any{
			"mediaID": id,
			"length":  len(text),
		},
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"text": text,
	})
}
