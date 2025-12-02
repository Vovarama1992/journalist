package domain

import (
	"context"
	"log"
	"time"

	"github.com/Vovarama1992/journalist/internal/domain/stations"
	"github.com/Vovarama1992/journalist/internal/models"
	"github.com/Vovarama1992/journalist/internal/ports"
)

type MediaService struct {
	repo ports.MediaRepository
	s1   *stations.S1ResolveURL
	s2   *stations.S2GrabPCM
	s3   *stations.S3PCMtoWAV
	s4   *stations.S4WAVtoText
	gpt  interface {
		Generate(ctx context.Context, previous string, nextRaw string) (string, error)
	}
	events chan ports.ChunkEvent
}

func NewMediaService(
	repo ports.MediaRepository,
	s1 *stations.S1ResolveURL,
	s2 *stations.S2GrabPCM,
	s3 *stations.S3PCMtoWAV,
	s4 *stations.S4WAVtoText,
	gpt interface {
		Generate(ctx context.Context, previous string, nextRaw string) (string, error)
	},
) *MediaService {
	return &MediaService{
		repo:   repo,
		s1:     s1,
		s2:     s2,
		s3:     s3,
		s4:     s4,
		gpt:    gpt,
		events: make(chan ports.ChunkEvent, 100),
	}
}

func (s *MediaService) Events() <-chan ports.ChunkEvent { return s.events }

func (s *MediaService) Process(
	ctx context.Context,
	srcURL string,
	roomID string,
) (*models.Media, error) {

	log.Printf("[MEDIA] start srcURL=%s roomID=%s", srcURL, roomID)

	media, err := s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: srcURL,
		Type:      "audio",
	})
	if err != nil {
		return nil, err
	}

	go func() {
		chunkNum := 1

		for {
			select {
			case <-ctx.Done():
				log.Printf("[MEDIA] ctx done → stop loop")
				return

			default:
				// ========== S1 ==========
				audioURL, err := s.s1.Run(ctx, srcURL)
				if err != nil {
					log.Printf("[MEDIA][ERR][S1] %v", err)
					time.Sleep(1 * time.Second)
					continue
				}

				// ========== S2 ==========
				pcm, err := s.s2.Run(ctx, audioURL)
				if err != nil {
					log.Printf("[MEDIA][ERR][S2] %v", err)
					continue
				}
				if len(pcm) == 0 {
					log.Printf("[MEDIA][WARN] empty pcm, chunk=%d", chunkNum)
					chunkNum++
					continue
				}

				// ========== S3 ==========
				wav, err := s.s3.Run(pcm)
				if err != nil {
					log.Printf("[MEDIA][ERR][S3] %v", err)
					continue
				}

				// ========== S4 ==========
				rawText, err := s.s4.Run(ctx, wav)
				if err != nil {
					log.Printf("[MEDIA][ERR][S4] %v", err)
					continue
				}
				if rawText == "" {
					log.Printf("[MEDIA][WARN] raw empty, chunk=%d", chunkNum)
					chunkNum++
					continue
				}

				// ========== GET PREVIOUS ==========
				prev, err := s.repo.GetLastChunk(ctx, media.ID)
				if err != nil {
					log.Printf("[MEDIA][ERR] get last chunk: %v", err)
					// продолжаем, prev = ""
				}

				// ========== GPT PROCESS ==========
				var prevText string
				if prev != nil {
					prevText = prev.Text
				}

				processed, err := s.gpt.Generate(ctx, prevText, rawText)
				if err != nil {
					log.Printf("[MEDIA][ERR][GPT] %v", err)
					continue
				}

				// ========== SAVE TO DB ==========
				err = s.repo.InsertChunk(ctx, &models.MediaChunk{
					MediaID:     media.ID,
					ChunkNumber: chunkNum,
					Text:        processed,
				})
				if err != nil {
					log.Printf("[MEDIA][ERR] insert chunk: %v", err)
					continue
				}

				// ========== SEND WS ==========
				s.events <- ports.ChunkEvent{
					MediaID:     media.ID,
					ChunkNumber: chunkNum,
					RoomID:      roomID,
					Text:        processed,
				}

				log.Printf("[MEDIA] OK chunk=%d text=%.60s", chunkNum, processed)
				chunkNum++
			}
		}
	}()

	return media, nil
}
