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

		hub.Register(roomID, conn)
		defer hub.Unregister(roomID)

		// ---------------------------------------------------
		// читаем URL от клиента
		// ---------------------------------------------------
		_, urlBytes, err := conn.ReadMessage()
		if err != nil {
			log.Printf("read url failed: %v", err)
			return
		}
		url := string(urlBytes)

		// первый пуш клиенту
		hub.SendToRoom(roomID, []byte(`{"status":"processing_started"}`))

		// ---------------------------------------------------
		// запускаем процесс медиа
		// ---------------------------------------------------
		go func() {
			_, err := mediaService.ProcessMedia(r.Context(), url, "audio", roomID)
			if err != nil {
				log.Printf("process media error: %v", err)
			}
			hub.SendToRoom(roomID, []byte(`{"status":"processing_finished"}`))
		}()

		// ---------------------------------------------------
		// держим соединение открытым, чтобы клиент не отвалился
		// ---------------------------------------------------
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}
}
