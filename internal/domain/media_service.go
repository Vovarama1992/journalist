package domain

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
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
// YOUTUBE RESOLVER
///////////////////////////////////////////////////////////////////////

///////////////////////////////////////////////////////////////////////
// 1) startFFmpeg — выдаём НЕ OPUS, а PCM (WAV RAW), чтобы Yandex не ругался
///////////////////////////////////////////////////////////////////////

func (s *MediaService) startFFmpeg(ctx context.Context, url string) (*bufio.Reader, *exec.Cmd, error) {
	log.Printf("[media] ffmpeg start (PCM): %.60s…", url)

	// PCM — чтоб 100% Yandex ел любые куски
	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-i", url,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-f", "s16le", // RAW PCM 16-bit signed
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
// 2) Это основной цикл чтения чанков каждые 10 секунд
///////////////////////////////////////////////////////////////////////

func (s *MediaService) readChunksLoop(
	ctx context.Context,
	reader io.Reader,
	media *models.Media,
	roomID string,
) {

	const chunkDuration = 10 * time.Second
	tmp := make([]byte, 64*1024)

	buf := &bytes.Buffer{}
	chunkNum := 0

	timer := time.NewTimer(chunkDuration)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[media] ctx done")
			return

		case <-timer.C:
			raw := buf.Bytes()
			chunkNum++

			log.Printf("[media] TIME-CUT chunk=%d raw=%d bytes", chunkNum, len(raw))

			if len(raw) > 0 {
				// отправляем на STT
				text, err := s.stt.Recognize(ctx, raw)
				if err != nil {
					log.Printf("[media] STT error chunk=%d: %v", chunkNum, err)
				} else if strings.TrimSpace(text) != "" {
					// сохраняем в БД
					ch := &models.MediaChunk{
						MediaID:     media.ID,
						ChunkNumber: chunkNum,
						Text:        text,
					}
					_ = s.repo.InsertChunk(ctx, ch)

					// шлём наружу
					s.events <- ports.ChunkEvent{
						RoomID:      roomID,
						MediaID:     media.ID,
						ChunkNumber: chunkNum,
						Text:        text,
					}
					log.Printf("[media] SEND chunk=%d text-len=%d", chunkNum, len(text))
				} else {
					log.Printf("[media] empty-text chunk=%d", chunkNum)
				}
			}

			// сбрасываем буфер
			buf.Reset()
			timer.Reset(chunkDuration)

		default:
			// читаем ffmpeg
			n, err := reader.Read(tmp)
			if n > 0 {
				buf.Write(tmp[:n])
			}
			if err != nil {
				if err == io.EOF {
					log.Printf("[media] ffmpeg EOF — finish")
					return
				}
				log.Printf("[media] ffmpeg read error: %v", err)
				return
			}
		}
	}
}

///////////////////////////////////////////////////////////////////////
// 3) ProcessMedia — обвязка
///////////////////////////////////////////////////////////////////////

func (s *MediaService) ProcessMedia(ctx context.Context, sourceURL, mediaType, roomID string) (*models.Media, error) {

	log.Printf("[media] start: %.60s…", sourceURL)

	// youtube
	if strings.Contains(sourceURL, "youtube") || strings.Contains(sourceURL, "youtu.be") {
		log.Printf("[media] youtube detected, resolving…")
		u, err := ResolveYouTube(sourceURL)
		if err != nil {
			return nil, fmt.Errorf("resolve youtube failed: %w", err)
		}
		log.Printf("[media] resolved: %.60s…", u)
		sourceURL = u
	}

	// create media DB
	media, err := s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: sourceURL,
		Type:      mediaType,
	})
	if err != nil {
		return nil, err
	}

	// FFmpeg
	reader, cmd, err := s.startFFmpeg(ctx, sourceURL)
	if err != nil {
		return nil, err
	}

	// читаем chunk-циклом
	go func() {
		s.readChunksLoop(ctx, reader, media, roomID)
		cmd.Wait()
		log.Printf("[media] finished mediaID=%d", media.ID)
	}()

	return media, nil
}
