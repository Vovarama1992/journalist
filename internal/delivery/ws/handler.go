package ws

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Vovarama1992/journalist/internal/ports"
)

type startMsg struct {
	URL     string `json:"url"`
	MediaID int    `json:"mediaID"`
}

func WSHandler(
	hub *Hub,
	media ports.MediaProcessor,
	ctxWS context.Context,
	cancelWS context.CancelFunc,
) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		conn, err := Upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, "ws upgrade failed", http.StatusBadRequest)
			return
		}

		roomID := r.URL.Query().Get("roomID")
		if roomID == "" {
			roomID = "default"
		}

		// минимальный лог
		println("[WS] start room:", roomID)
		hub.Register(roomID, conn)

		defer func() {
			cancelWS()
			println("[WS] end room:", roomID)
			hub.Unregister(roomID, conn)
		}()

		_, raw, err := conn.ReadMessage()
		if err != nil {
			println("[WS] read init fail")
			return
		}

		var req startMsg
		if err := json.Unmarshal(raw, &req); err != nil {
			println("[WS] bad json")
			hub.SendToRoom(roomID, []byte(`{"status":"error"}`))
			return
		}

		// лог только факта
		println("[WS] init url:", req.URL, "mediaID:", req.MediaID)
		hub.SendToRoom(roomID, []byte(`{"status":"processing_started"}`))

		go func() {
			mediaObj, err := media.Process(ctxWS, req.URL, roomID, req.MediaID)
			if err != nil {
				println("[WS] media error")
				hub.SendToRoom(roomID, []byte(`{"status":"error"}`))
				return
			}

			resp := map[string]any{
				"status":  "ok",
				"mediaID": mediaObj.ID,
			}
			b, _ := json.Marshal(resp)
			hub.SendToRoom(roomID, b)
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				println("[WS] disconnect room:", roomID)
				return
			}
		}
	}
}
