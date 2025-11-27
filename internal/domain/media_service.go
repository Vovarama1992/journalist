package domain

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os/exec"

	"github.com/Vovarama1992/journalist/internal/models"
	"github.com/Vovarama1992/journalist/internal/ports"
)

type MediaService struct {
	repo ports.MediaRepository
	stt  ports.STTService
}

func NewMediaService(repo ports.MediaRepository, stt ports.STTService) *MediaService {
	return &MediaService{
		repo: repo,
		stt:  stt,
	}
}

// ProcessMedia: создаёт media, нарезает на чанки, сохраняет пустые чанки, отправляет в STT и обновляет текст
func (s *MediaService) ProcessMedia(ctx context.Context, sourceURL, mediaType string) (*models.Media, error) {
	media, err := s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: sourceURL,
		Type:      mediaType,
	})
	if err != nil {
		return nil, fmt.Errorf("create media: %w", err)
	}

	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-i", sourceURL,
		"-f", "segment",
		"-segment_time", "15",
		"-c", "copy",
		"-map", "0:a",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}

	reader := bufio.NewReader(stdout)
	chunkNum := 0

	for {
		buf := make([]byte, 32*1024)
		n, err := reader.Read(buf)
		if n > 0 {
			chunkNum++
			chunk := &models.MediaChunk{
				MediaID:     media.ID,
				ChunkNumber: chunkNum,
				Data:        buf[:n],
			}

			if err := s.repo.InsertChunk(ctx, chunk); err != nil {
				log.Printf("insert chunk failed: %v", err)
			}

			go func(c *models.MediaChunk) {
				text, err := s.stt.Recognize(ctx, c.Data)
				if err != nil {
					log.Printf("STT error: %v", err)
					return
				}
				if err := s.repo.UpdateChunkText(ctx, c.ID, text); err != nil {
					log.Printf("update chunk text failed: %v", err)
				}
			}(chunk)
		}
		if err != nil {
			break
		}
	}

	cmd.Wait()
	return media, nil
}
