package delivery

import (
	"log"
	"net/http"

	"github.com/Vovarama1992/journalist/internal/domain"
)

// WSHandler запускает ProcessMedia на MediaService
func WSHandler(hub *Hub, mediaService *domain.MediaService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// апгрейд соединения
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

		// читаем первое сообщение — URL
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("read url failed: %v", err)
			return
		}
		url := string(msg)
		log.Printf("received URL from client: %s", url)

		// подтверждение старта обработки
		ack := []byte("processing started")
		if err := hub.SendToRoom(roomID, ack); err != nil {
			log.Printf("send ack failed: %v", err)
		}

		// запускаем ProcessMedia в отдельной горутине
		go func() {
			_, err := mediaService.ProcessMedia(r.Context(), url, "audio")
			if err != nil {
				log.Printf("process media failed: %v", err)
			}
			// можно добавить отправку финального уведомления на фронт
			done := []byte("processing finished")
			hub.SendToRoom(roomID, done)
		}()

		// оставляем соединение открытым
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}
}
