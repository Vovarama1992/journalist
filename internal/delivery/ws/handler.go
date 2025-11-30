package ws

import (
	"log"
	"net/http"

	"github.com/Vovarama1992/journalist/internal/ports"
)

func WSHandler(hub *Hub, media ports.MediaProcessor) http.HandlerFunc {
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
		defer hub.Unregister(roomID)

		_, urlBytes, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WS][IN] read fail: %v", err)
			return
		}
		url := string(urlBytes)

		log.Printf("[WS][INSIDE] url=%s", url)
		hub.SendToRoom(roomID, []byte(`{"status":"processing_started"}`))

		go func() {
			_, err := media.Process(r.Context(), url, roomID)
			if err != nil {
				log.Printf("[WS][OUT] media error: %v", err)
				hub.SendToRoom(roomID, []byte(`{"status":"error"}`))
				return
			}
			hub.SendToRoom(roomID, []byte(`{"status":"processing_finished"}`))
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				log.Printf("[WS][OUT] disconnected room=%s", roomID)
				return
			}
		}
	}
}
