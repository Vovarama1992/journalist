package domain

import (
	"context"
	"fmt"
	"log"

	"github.com/Vovarama1992/journalist/internal/domain/stations"
	"github.com/Vovarama1992/journalist/internal/models"
	"github.com/Vovarama1992/journalist/internal/ports"
)

type MediaService struct {
	repo ports.MediaRepository

	s1 *stations.S1ResolveURL
	s2 *stations.S2GrabPCM
	s3 *stations.S3PCMtoWAV
	s4 *stations.S4WAVtoText
	s5 *stations.S5GPT

	events chan ports.ChunkEvent
}

func NewMediaService(
	repo ports.MediaRepository,
	s1 *stations.S1ResolveURL,
	s2 *stations.S2GrabPCM,
	s3 *stations.S3PCMtoWAV,
	s4 *stations.S4WAVtoText,
	gpt ports.GPTService,
) *MediaService {
	return &MediaService{
		repo:   repo,
		s1:     s1,
		s2:     s2,
		s3:     s3,
		s4:     s4,
		s5:     stations.NewS5GPT(gpt),
		events: make(chan ports.ChunkEvent, 100),
	}
}

func (m *MediaService) Events() <-chan ports.ChunkEvent {
	return m.events
}

func (m *MediaService) Process(
	ctx context.Context,
	srcURL string,
	roomID string,
	mediaID int,
) (*models.Media, error) {

	var media *models.Media
	var err error

	// --- CASE A: RESUME ---
	if mediaID > 0 {
		media, err = m.repo.GetMediaByID(ctx, mediaID)
		if err != nil {
			return nil, err
		}
		if media == nil {
			return nil, fmt.Errorf("media %d not found", mediaID)
		}

		log.Printf("[MEDIA] resume mediaID=%d", mediaID)

		// ключевой фикс:
		srcURL = media.SourceURL
	}

	// --- CASE B: NEW ---
	if mediaID == 0 {
		media, err = m.repo.InsertMedia(ctx, &models.Media{
			SourceURL: srcURL,
			Type:      "audio",
		})
		if err != nil {
			return nil, err
		}
		log.Printf("[MEDIA] new mediaID=%d", media.ID)
	}

	// ------------------------
	// PIPELINE
	// ------------------------
	go func() {
		chunk := 1

		// resume продолжает нумерацию
		if mediaID > 0 {
			last, _ := m.repo.GetLastChunk(ctx, media.ID)
			if last != nil {
				chunk = last.ChunkNumber + 1
			}
		}

		for {
			select {
			case <-ctx.Done():
				log.Printf("[MEDIA] stop")
				return
			default:
			}

			log.Printf("[MEDIA] loop chunk=%d srcURL=%s", chunk, srcURL)

			audioURL, err := m.s1.Run(ctx, srcURL)
			if err != nil {
				log.Printf("[MEDIA] S1 err=%v", err)
				continue
			}

			pcm, err := m.s2.Run(ctx, audioURL)
			if err != nil || len(pcm) == 0 {
				log.Printf("[MEDIA] S2 err=%v", err)
				continue
			}

			wav := m.s3.Run(pcm)

			raw, err := m.s4.Run(ctx, wav)
			if err != nil || raw == "" {
				log.Printf("[MEDIA] S4 empty")
				continue
			}

			prev, _ := m.repo.GetLastChunk(ctx, media.ID)
			prevText := ""
			if prev != nil {
				prevText = prev.Text
			}

			proc, err := m.s5.Run(ctx, prevText, raw)
			if err != nil || proc == "" {
				log.Printf("[MEDIA] GPT FAIL — using raw text")

				proc = raw // используем S4 без обработки
			}

			err = m.repo.InsertChunk(ctx, &models.MediaChunk{
				MediaID:     media.ID,
				ChunkNumber: chunk,
				Text:        proc,
			})
			if err != nil {
				log.Printf("[MEDIA] save err=%v", err)
				continue
			}

			m.events <- ports.ChunkEvent{
				MediaID:     media.ID,
				ChunkNumber: chunk,
				RoomID:      roomID,
				Text:        proc,
			}

			log.Printf("[MEDIA] OK chunk=%d", chunk)
			chunk++
		}
	}()

	return media, nil
}
