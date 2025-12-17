package delivery

import (
	"encoding/json"
	"net/http"

	"github.com/Vovarama1992/go-utils/logger"
	"github.com/Vovarama1992/journalist/internal/ports"
)

type AuthHandler struct {
	auth ports.AuthService
	log  *logger.ZapLogger
}

func NewAuthHandler(auth ports.AuthService, log *logger.ZapLogger) *AuthHandler {
	return &AuthHandler{
		auth: auth,
		log:  log,
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}

	token, err := h.auth.Login(r.Context(), req.Password)
	if err != nil {
		http.Error(w, "invalid password", http.StatusUnauthorized)
		return
	}

	h.log.Log(logger.LogEntry{
		Level:   "info",
		Message: "login success",
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"token": token,
	})
}
