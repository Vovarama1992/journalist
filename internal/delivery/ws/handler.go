package ws

import (
	"context"
	"encoding/json"
	"log"
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

		log.Printf("[WS][IN] start room=%s", roomID)
		hub.Register(roomID, conn)

		// правильный defer
		defer func() {
			cancelWS() // <- ключевой фикс
			log.Printf("[WS][OUT] room=%s", roomID)
			hub.Unregister(roomID, conn)
		}()
		// читаем init
		_, raw, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WS][IN] read fail: %v", err)
			return
		}

		var req startMsg
		if err := json.Unmarshal(raw, &req); err != nil {
			log.Printf("[WS] bad json: %v", err)
			hub.SendToRoom(roomID, []byte(`{"status":"error"}`))
			return
		}

		log.Printf("[WS][INSIDE] url=%s mediaID=%d", req.URL, req.MediaID)
		hub.SendToRoom(roomID, []byte(`{"status":"processing_started"}`))

		// pipeline
		go func() {
			mediaObj, err := media.Process(ctxWS, req.URL, roomID, req.MediaID)
			if err != nil {
				log.Printf("[WS][OUT] media error: %v", err)
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

		// держим соединение живым
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				log.Printf("[WS][OUT] disconnect room=%s", roomID)
				return
			}
		}
	}
}
