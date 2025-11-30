package domain

import (
	"context"
	"encoding/binary"
	"io"
	"log"
	"os/exec"
	"strings"

	"github.com/Vovarama1992/journalist/internal/models"
	"github.com/Vovarama1992/journalist/internal/ports"
)

type MediaServiceA struct {
	repo   ports.MediaRepository
	stt    ports.STTService
	events chan ports.ChunkEvent
}

func NewMediaServiceA(repo ports.MediaRepository, stt ports.STTService) *MediaServiceA {
	return &MediaServiceA{
		repo:   repo,
		stt:    stt,
		events: make(chan ports.ChunkEvent, 100),
	}
}

func (s *MediaServiceA) Events() <-chan ports.ChunkEvent {
	return s.events
}

// ============================
// PCM → WAV (logged)
// ============================
func pcmToWavA(pcm []byte) []byte {
	log.Printf("[A-S3] PCM→WAV: pcm=%d bytes", len(pcm))

	const (
		audioFormat   = 1
		numChannels   = 1
		sampleRate    = 16000
		bitsPerSample = 16
	)
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataSize := len(pcm)
	riffSize := 36 + dataSize

	h := make([]byte, 44)
	copy(h[0:], []byte("RIFF"))
	binary.LittleEndian.PutUint32(h[4:], uint32(riffSize))
	copy(h[8:], []byte("WAVE"))
	copy(h[12:], []byte("fmt "))
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], audioFormat)
	binary.LittleEndian.PutUint16(h[22:], numChannels)
	binary.LittleEndian.PutUint32(h[24:], sampleRate)
	binary.LittleEndian.PutUint32(h[28:], uint32(byteRate))
	binary.LittleEndian.PutUint16(h[32:], uint16(blockAlign))
	binary.LittleEndian.PutUint16(h[34:], bitsPerSample)
	copy(h[36:], []byte("data"))
	binary.LittleEndian.PutUint32(h[40:], uint32(dataSize))

	wav := append(h, pcm...)
	log.Printf("[A-S3 OUT] WAV ready: %d bytes", len(wav))
	return wav
}

// ============================
// S1 — grabOneChunk
// ============================
func (s *MediaServiceA) grabOneChunk(ctx context.Context, url string, bytes int) ([]byte, error) {
	log.Printf("[A-S1 IN] url=%s want=%d", url, bytes)
	log.Printf("[A-S1 STEP] spawn ffmpeg -t 5")

	cmd := exec.CommandContext(
		ctx, "ffmpeg",
		"-re",
		"-seekable", "0",
		"-i", url,
		"-t", "5",
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-f", "s16le",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("[A-S1 ERR] stdout pipe %v", err)
		return nil, err
	}

	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		log.Printf("[A-S1 ERR] start %v", err)
		return nil, err
	}

	// stderr watcher
	go func() {
		slurp, _ := io.ReadAll(stderr)
		if len(slurp) > 0 {
			log.Printf("[A-S1 FFMPEG STDERR] %s", slurp)
		}
	}()

	buf := make([]byte, bytes)
	n, err := io.ReadFull(stdout, buf)
	if err != nil {
		log.Printf("[A-S1 ERR] readFull: %v", err)
		return nil, err
	}

	log.Printf("[A-S1 STEP] read %d/%d bytes", n, bytes)
	cmd.Wait()

	log.Printf("[A-S1 OUT] pcm chunk OK")
	return buf, nil
}

// ============================
// S3 — STT
// ============================
func (s *MediaServiceA) sttCall(ctx context.Context, wav []byte) (string, error) {
	log.Printf("[A-S3 IN] wav=%d bytes", len(wav))
	txt, err := s.stt.Recognize(ctx, wav)
	if err != nil {
		log.Printf("[A-S3 ERR] %v", err)
		return "", err
	}
	log.Printf("[A-S3 OUT] raw=%.50s", txt)
	return txt, nil
}

// ============================
// ORCHESTRATOR
// ============================
func (s *MediaServiceA) ProcessMedia(ctx context.Context, src, mediaType, roomID string) (*models.Media, error) {

	log.Printf("[A-ORCH IN] src=%s", src)

	media, err := s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: src,
		Type:      mediaType,
	})
	if err != nil {
		log.Printf("[A-ORCH ERR] insert media %v", err)
		return nil, err
	}

	go func() {
		chunk := 1
		for {
			select {
			case <-ctx.Done():
				log.Printf("[A-ORCH] ctx done, exit")
				return

			default:
				// S1 — ffmpeg на 5 секунд
				log.Printf("[A-ORCH] S1 grab chunk %d", chunk)
				pcm, err := s.grabOneChunk(ctx, src, 160000)
				if err != nil {
					log.Printf("[A-ORCH WARN] S1 failed, retry… %v", err)
					continue
				}

				// PCM → WAV
				wav := pcmToWavA(pcm)

				// S3 — STT
				txt, err := s.sttCall(ctx, wav)
				if err != nil {
					log.Printf("[A-ORCH WARN] S3 err, skip chunk %d", chunk)
					chunk++
					continue
				}

				txt = strings.TrimSpace(txt)
				log.Printf("[A-ORCH STEP] text=%.50s", txt)
				if txt == "" {
					log.Printf("[A-ORCH WARN] empty text, skip")
					chunk++
					continue
				}

				// Save
				s.repo.InsertChunk(ctx, &models.MediaChunk{
					MediaID:     media.ID,
					ChunkNumber: chunk,
					Text:        txt,
				})

				log.Printf("[A-ORCH SEND] chunk=%d text=%.40s", chunk, txt)

				s.events <- ports.ChunkEvent{
					MediaID:     media.ID,
					ChunkNumber: chunk,
					RoomID:      roomID,
					Text:        txt,
				}

				chunk++
			}
		}
	}()

	return media, nil
}
