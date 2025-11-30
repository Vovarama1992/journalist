package domain

import (
	"bytes"
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

// AggressiveMediaService — "под каждый чанк новое подключение".
type AggressiveMediaService struct {
	repo   ports.MediaRepository
	stt    ports.STTService
	events chan ports.ChunkEvent
}

func NewAggressiveMediaService(repo ports.MediaRepository, stt ports.STTService) *AggressiveMediaService {
	return &AggressiveMediaService{
		repo:   repo,
		stt:    stt,
		events: make(chan ports.ChunkEvent, 100),
	}
}

func (s *AggressiveMediaService) Events() <-chan ports.ChunkEvent {
	return s.events
}

// -----------------------------
// СТАНЦИЯ 1 — исходный URL стрима → прямой audio-URL (через yt-dlp)
// -----------------------------

func (s *AggressiveMediaService) stationResolveURL(ctx context.Context, pageURL string) (string, error) {
	out, err := exec.CommandContext(
		ctx,
		"yt-dlp",
		"--no-playlist",
		"-g",
		pageURL,
	).CombinedOutput()

	raw := strings.TrimSpace(string(out))
	log.Printf("[S1 RESOLVE] raw=%q err=%v", raw, err)

	if err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w (%s)", err, raw)
	}

	if raw == "" || !strings.HasPrefix(raw, "http") {
		return "", fmt.Errorf("invalid audio url: %q", raw)
	}

	return raw, nil
}

// -----------------------------
// СТАНЦИЯ 2 — прямой audio-URL → 5 секунд PCM (s16le 16kHz mono)
// -----------------------------

func (s *AggressiveMediaService) stationGrabPCM(ctx context.Context, audioURL string) ([]byte, error) {
	log.Printf("[S2 IN] audioURL=%s", audioURL)

	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-i", audioURL,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-t", "5",
		"-f", "s16le",
		"pipe:1",
	)

	log.Printf("[S2 INSIDE] run: ffmpeg -i <audio> -ac 1 -ar 16000 -t 5 -f s16le pipe:1")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("[S2 OUT] stdout pipe: %w", err)
	}
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("[S2 OUT] ffmpeg start: %w", err)
	}

	// логируем stderr целиком
	go func() {
		b, _ := io.ReadAll(stderr)
		if len(b) > 0 {
			log.Printf("[S2 INSIDE FFMPEG STDERR] %s", b)
		}
	}()

	pcm, err := io.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("[S2 OUT] read pcm: %w", err)
	}

	log.Printf("[S2 INSIDE] pcmBytes=%d", len(pcm))
	if len(pcm) > 0 {
		previewN := 32
		if len(pcm) < previewN {
			previewN = len(pcm)
		}
		log.Printf("[S2 INSIDE] pcm[0:%d]=%v", previewN, pcm[:previewN])
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("[S2 INSIDE] ffmpeg exit err=%v", err)
		// всё равно считаем кусок валидным, если байты есть
	}

	log.Printf("[S2 OUT] pcm=%d bytes", len(pcm))
	return pcm, nil
}

// -----------------------------
// СТАНЦИЯ 3 — PCM → WAV (RIFF header)
// -----------------------------

func (s *AggressiveMediaService) stationPCMtoWAV(pcm []byte) []byte {
	log.Printf("[S3 IN] pcm=%d", len(pcm))

	const (
		sampleRate     = 16000
		channels       = 1
		bitsPerSample  = 16
		bytesPerSample = bitsPerSample / 8
	)

	dataSize := len(pcm)
	byteRate := sampleRate * channels * bytesPerSample
	blockAlign := channels * bytesPerSample

	buf := &bytes.Buffer{}

	// RIFF header
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16)) // PCM fmt chunk size
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))  // audio format = PCM
	_ = binary.Write(buf, binary.LittleEndian, uint16(channels))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	_ = binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	_ = binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))

	// data chunk
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	buf.Write(pcm)

	wav := buf.Bytes()

	log.Printf("[S3 INSIDE] wavBytes=%d", len(wav))
	previewN := 64
	if len(wav) < previewN {
		previewN = len(wav)
	}
	if previewN > 0 {
		log.Printf("[S3 INSIDE] wav[0:%d]=%v", previewN, wav[:previewN])
	}

	log.Printf("[S3 OUT] wav=%d", len(wav))
	return wav
}

// -----------------------------
// СТАНЦИЯ 4 — WAV → TEXT (Yandex STT)
// -----------------------------

func (s *AggressiveMediaService) stationSTT(ctx context.Context, wav []byte) (string, error) {
	log.Printf("[S4 IN] wav=%d", len(wav))

	txt, err := s.stt.Recognize(ctx, wav)

	log.Printf("[S4 INSIDE] rawTxt=%q err=%v", txt, err)
	if err != nil {
		return "", fmt.Errorf("[S4 OUT] stt err: %w", err)
	}

	txt = strings.TrimSpace(txt)
	log.Printf("[S4 OUT] txt=%q", txt)
	return txt, nil
}

// -----------------------------
// ОРКЕСТР — пока жив контекст, циклимся
// -----------------------------

func (s *AggressiveMediaService) Process(
	ctx context.Context,
	srcURL string,
	roomID string,
) (*models.Media, error) {

	log.Printf("[MEDIA AGGR IN] srcURL=%s roomID=%s", srcURL, roomID)

	// mediaType в интерфейсе отсутствует — жёстко ставим "audio"
	media, err := s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: srcURL,
		Type:      "audio",
	})
	if err != nil {
		return nil, fmt.Errorf("[MEDIA AGGR OUT] insert media err: %w", err)
	}

	go func() {
		chunkNum := 1

		for {
			select {
			case <-ctx.Done():
				log.Printf("[MEDIA AGGR INSIDE] ctx done, stop loop")
				return

			default:
				log.Printf("[MEDIA AGGR INSIDE] loop chunk=%d", chunkNum)

				// S1: page URL → audio URL
				audioURL, err := s.stationResolveURL(ctx, srcURL)
				if err != nil {
					log.Printf("[MEDIA AGGR] S1 err: %v", err)
					continue
				}

				// S2: audio URL → PCM-кусок
				pcm, err := s.stationGrabPCM(ctx, audioURL)
				if err != nil {
					log.Printf("[MEDIA AGGR] S2 err: %v", err)
					continue
				}
				if len(pcm) == 0 {
					log.Printf("[MEDIA AGGR] S2 got EMPTY PCM chunk=%d", chunkNum)
					chunkNum++
					continue
				}

				// S3: PCM → WAV
				wav := s.stationPCMtoWAV(pcm)

				// S4: WAV → текст
				txt, err := s.stationSTT(ctx, wav)
				if err != nil {
					log.Printf("[MEDIA AGGR] S4 err: %v", err)
					chunkNum++
					continue
				}
				if txt == "" {
					log.Printf("[MEDIA AGGR] S4 empty text chunk=%d", chunkNum)
					chunkNum++
					continue
				}

				// Сохраняем
				err = s.repo.InsertChunk(ctx, &models.MediaChunk{
					MediaID:     media.ID,
					ChunkNumber: chunkNum,
					Text:        txt,
				})
				if err != nil {
					log.Printf("[MEDIA AGGR] insert chunk err: %v", err)
				}

				log.Printf("[MEDIA AGGR OUT] chunk=%d text=%.80s", chunkNum, txt)

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
