package domain

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"

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

// FFmpeg → PCM stream
func (s *MediaService) continuousCapture(ctx context.Context, url string) (io.ReadCloser, error) {
	log.Printf("[stream] continuousCapture START url=%.80s", url)

	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-re",
		"-seekable", "0",
		"-i", url,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-f", "s16le",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ffmpeg start: %w", err)
	}

	go func() {
		errLog, _ := io.ReadAll(stderr)
		if len(errLog) > 0 {
			log.Printf("[stream] ffmpeg stderr: %s", errLog)
		}
		log.Printf("[stream] ffmpeg finished")
	}()

	return stdout, nil
}

// PCM → WAV (добавляем WAV-хедер)
func pcmToWav(pcm []byte) ([]byte, error) {
	dataSize := uint32(len(pcm))
	riffSize := 36 + dataSize

	buf := make([]byte, 44+len(pcm))

	copy(buf[0:], []byte("RIFF"))
	binary.LittleEndian.PutUint32(buf[4:], riffSize)
	copy(buf[8:], []byte("WAVE"))

	copy(buf[12:], []byte("fmt "))
	binary.LittleEndian.PutUint32(buf[16:], 16)      // PCM header size
	binary.LittleEndian.PutUint16(buf[20:], 1)       // PCM = 1
	binary.LittleEndian.PutUint16(buf[22:], 1)       // channels = 1
	binary.LittleEndian.PutUint32(buf[24:], 16000)   // sample rate
	binary.LittleEndian.PutUint32(buf[28:], 16000*2) // byte rate
	binary.LittleEndian.PutUint16(buf[32:], 2)       // block align
	binary.LittleEndian.PutUint16(buf[34:], 16)      // bits per sample

	copy(buf[36:], []byte("data"))
	binary.LittleEndian.PutUint32(buf[40:], dataSize)

	copy(buf[44:], pcm)

	return buf, nil
}

func (s *MediaService) ProcessMedia(ctx context.Context, sourceURL, mediaType, roomID string) (*models.Media, error) {
	log.Printf("[media] start PTRP: %.60s…", sourceURL)

	media, err := s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: sourceURL,
		Type:      mediaType,
	})
	if err != nil {
		return nil, err
	}

	stream, err := s.continuousCapture(ctx, sourceURL)
	if err != nil {
		return nil, fmt.Errorf("continuousCapture: %w", err)
	}

	go func() {
		defer stream.Close()

		const chunkBytes = 160000
		buf := make([]byte, chunkBytes)
		chunkNum := 1

		for {
			select {
			case <-ctx.Done():
				log.Printf("[media] ctx done — stop PTRP")
				return

			default:
				n, err := io.ReadFull(stream, buf)
				if err != nil {
					log.Printf("[media] readFull err: %v", err)
					return
				}
				if n < chunkBytes {
					log.Printf("[media] short read %d bytes", n)
					continue
				}

				// *** ВСТАВЛЕННАЯ СТАНЦИЯ PCM → WAV ***
				wav, err := pcmToWav(buf)
				if err != nil {
					log.Printf("[media] pcmToWav err chunk=%d: %v", chunkNum, err)
					chunkNum++
					continue
				}

				// WAV → STT
				txt, err := s.stt.Recognize(ctx, wav)
				if err != nil {
					log.Printf("[media] STT err chunk=%d: %v", chunkNum, err)
					chunkNum++
					continue
				}

				txt = strings.TrimSpace(txt)
				if txt == "" {
					log.Printf("[media] empty text chunk=%d", chunkNum)
					chunkNum++
					continue
				}

				_ = s.repo.InsertChunk(ctx, &models.MediaChunk{
					MediaID:     media.ID,
					ChunkNumber: chunkNum,
					Text:        txt,
				})

				log.Printf("[media] SEND chunk=%d media=%d: %.40s", chunkNum, media.ID, txt)

				s.events <- ports.ChunkEvent{
					MediaID:     media.ID,
					ChunkNumber: chunkNum,
					RoomID:      roomID,
					Text:        txt,
				}

				chunkNum++
			}
		}
	}()

	return media, nil
}
