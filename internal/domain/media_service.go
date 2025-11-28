package domain

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/Vovarama1992/journalist/internal/models"
	"github.com/Vovarama1992/journalist/internal/ports"
)

type MediaService struct {
	repo   ports.MediaRepository
	stt    ports.STTService
	events chan ports.ChunkEvent
}

func NewMediaService(repo ports.MediaRepository, stt ports.STTService) *MediaService {
	return &MediaService{
		repo:   repo,
		stt:    stt,
		events: make(chan ports.ChunkEvent, 100),
	}
}

func (s *MediaService) Events() <-chan ports.ChunkEvent {
	return s.events
}

///////////////////////////////////////////////////////////////////////
// 1) создать запись media
///////////////////////////////////////////////////////////////////////

func (s *MediaService) createMedia(ctx context.Context, src, typ string) (*models.Media, error) {
	return s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: src,
		Type:      typ,
	})
}

///////////////////////////////////////////////////////////////////////
// 2) запустить ffmpeg и получить поток PCM
///////////////////////////////////////////////////////////////////////

func (s *MediaService) startFFmpeg(ctx context.Context, url string) (*bufio.Reader, *exec.Cmd, error) {
	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-i", url,
		"-vn",
		"-ac", "1",
		"-ar", "48000",
		"-f", "s16le",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start ffmpeg: %w", err)
	}

	return bufio.NewReader(stdout), cmd, nil
}

///////////////////////////////////////////////////////////////////////
// 3) читать кусок данных из ffmpeg
///////////////////////////////////////////////////////////////////////

func (s *MediaService) readFromFFmpeg(reader *bufio.Reader, buf []byte) ([]byte, error) {
	tmp := make([]byte, 256*1024)
	n, err := reader.Read(tmp)
	if n > 0 {
		buf = append(buf, tmp[:n]...)
	}
	return buf, err
}

///////////////////////////////////////////////////////////////////////
// 4) сохранить чанк в БД и отправить в STT
///////////////////////////////////////////////////////////////////////

func (s *MediaService) saveAndProcessChunk(ctx context.Context, mediaID int, chunkNum int, audio []byte) {
	chunk := &models.MediaChunk{
		MediaID:     mediaID,
		ChunkNumber: chunkNum,
		Text:        "",
	}

	_ = s.repo.InsertChunk(ctx, chunk)

	go func(c *models.MediaChunk, data []byte) {
		text, err := s.stt.Recognize(ctx, data)
		if err != nil {
			log.Printf("STT error: %v", err)
			return
		}

		_ = s.repo.UpdateChunkText(ctx, c.ID, text)

		s.events <- ports.ChunkEvent{
			MediaID:     c.MediaID,
			ChunkNumber: c.ChunkNumber,
			Text:        text,
		}
	}(chunk, audio)
}

///////////////////////////////////////////////////////////////////////
// 5) ProcessMedia — главный оркестратор + RESOLVE YOUTUBE
///////////////////////////////////////////////////////////////////////

func (s *MediaService) ProcessMedia(ctx context.Context, sourceURL, mediaType string) (*models.Media, error) {

	// -------------------------------------------------
	// ВСТАВКА: РЕЗОЛВИНГ YOUTUBE (СТРОГО КАК ТЫ ХОТЕЛ)
	// -------------------------------------------------
	if strings.Contains(sourceURL, "youtube.com") || strings.Contains(sourceURL, "youtu.be") {
		u, err := ResolveYouTube(sourceURL)
		if err != nil {
			return nil, fmt.Errorf("resolve youtube failed: %w", err)
		}
		sourceURL = u
	}

	// -------------------------------------------------
	// 1) создаём запись media
	// -------------------------------------------------
	media, err := s.createMedia(ctx, sourceURL, mediaType)
	if err != nil {
		return nil, err
	}

	// -------------------------------------------------
	// 2) запускаем ffmpeg уже по RESOLVED URL
	// -------------------------------------------------
	reader, cmd, err := s.startFFmpeg(ctx, sourceURL)
	if err != nil {
		return nil, err
	}

	var buf []byte
	chunkNum := 0

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// каждые 15 секунд — формируем чанк
	go func() {
		for range ticker.C {
			if len(buf) == 0 {
				continue
			}

			chunkNum++
			audioCopy := buf
			buf = nil

			s.saveAndProcessChunk(ctx, media.ID, chunkNum, audioCopy)
		}
	}()

	for {
		buf, err = s.readFromFFmpeg(reader, buf)
		if err != nil {
			break
		}
	}

	cmd.Wait()
	return media, nil
}
