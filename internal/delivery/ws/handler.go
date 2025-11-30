package ws

import (
	"log"
	"net/http"

	"github.com/Vovarama1992/journalist/internal/domain"
)

func WSHandler(hub *Hub, mediaService *domain.MediaService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, "failed to upgrade", http.StatusBadRequest)
			return
		}

		roomID := r.URL.Query().Get("roomID")
		if roomID == "" {
			roomID = "default"
		}

		log.Printf("[WS] connected room=%s", roomID)

		hub.Register(roomID, conn)
		defer hub.Unregister(roomID)

		_, urlBytes, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WS] read url failed room=%s err=%v", roomID, err)
			return
		}
		url := string(urlBytes)

		log.Printf("[WS] room=%s url=%s", roomID, url)

		hub.SendToRoom(roomID, []byte(`{"status":"processing_started"}`))

		go func() {
			_, err := mediaService.ProcessMedia(r.Context(), url, "audio", roomID)
			if err != nil {
				log.Printf("[WS] process error room=%s err=%v", roomID, err)

				hub.SendToRoom(roomID, []byte(`{"status":"error", "message":"stream_failed"}`))

				log.Printf("[WS] sent error to room=%s", roomID)
				return
			}

			hub.SendToRoom(roomID, []byte(`{"status":"processing_finished"}`))
			log.Printf("[WS] finished room=%s", roomID)
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				log.Printf("[WS] disconnected room=%s", roomID)
				return
			}
		}
	}
}
