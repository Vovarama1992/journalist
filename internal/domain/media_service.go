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

type ConservativeMediaService struct {
	repo   ports.MediaRepository
	stt    ports.STTService
	events chan ports.ChunkEvent
}

func NewConservativeMediaService(repo ports.MediaRepository, stt ports.STTService) *ConservativeMediaService {
	return &ConservativeMediaService{
		repo:   repo,
		stt:    stt,
		events: make(chan ports.ChunkEvent, 100),
	}
}

func (s *ConservativeMediaService) Events() <-chan ports.ChunkEvent {
	return s.events
}

////////////////////////////////////////////////////////////////////////////////
// СТАНЦИЯ 0 — RESOLVE
////////////////////////////////////////////////////////////////////////////////

func (s *ConservativeMediaService) RESOLVE(ctx context.Context, raw string) (string, error) {
	log.Printf("[RESOLVE][IN] raw=%s", raw)

	if strings.Contains(raw, "youtube.com") || strings.Contains(raw, "youtu.be") {

		extract := func(format string) (string, error) {
			out, err := exec.CommandContext(
				ctx,
				"yt-dlp",
				"-f", format,
				"--extractor-args", "youtube:player_client=default",
				"--no-playlist",
				"-g",
				raw,
			).CombinedOutput()

			log.Printf("[RESOLVE][INSIDE] fmt=%s out=%q err=%v", format, out, err)

			if err != nil {
				return "", err
			}

			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			last := strings.TrimSpace(lines[len(lines)-1])

			if !strings.HasPrefix(last, "http") {
				return "", fmt.Errorf("invalid url line: %q", last)
			}

			return last, nil
		}

		if url, err := extract("bestaudio"); err == nil {
			log.Printf("[RESOLVE][OUT] bestaudio=%s", url)
			return url, nil
		}

		if url, err := extract("best"); err == nil {
			log.Printf("[RESOLVE][OUT] best=%s", url)
			return url, nil
		}

		return "", fmt.Errorf("yt-dlp failed for all formats")
	}

	log.Printf("[RESOLVE][OUT] passthrough=%s", raw)
	return raw, nil
}

////////////////////////////////////////////////////////////////////////////////
// СТАНЦИЯ 1 — PCM_START
////////////////////////////////////////////////////////////////////////////////

func (s *ConservativeMediaService) PCM_START(ctx context.Context, url string) (io.ReadCloser, error) {
	log.Printf("[PCM_START][IN] url=%s", url)

	cmd := exec.CommandContext(ctx,
		"ffmpeg", "-re", "-seekable", "0",
		"-i", url,
		"-vn",
		"-ac", "1", "-ar", "16000",
		"-f", "s16le", "pipe:1",
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
		errbuf, _ := io.ReadAll(stderr)
		if len(errbuf) > 0 {
			log.Printf("[PCM_START][INSIDE] ffmpeg stderr: %s", errbuf)
		}
		log.Printf("[PCM_START][OUT] ffmpeg exited")
	}()

	log.Printf("[PCM_START][OUT] stream opened")
	return stdout, nil
}

////////////////////////////////////////////////////////////////////////////////
// СТАНЦИЯ 2 — CUT
////////////////////////////////////////////////////////////////////////////////

func (s *ConservativeMediaService) CUT(r io.Reader) ([]byte, error) {
	log.Printf("[CUT][IN] reading pcm chunk")

	const N = 160000
	buf := make([]byte, N)

	n, err := io.ReadFull(r, buf)
	log.Printf("[CUT][INSIDE] n=%d err=%v", n, err)

	if err != nil {
		return nil, err
	}

	log.Printf("[CUT][OUT] chunk ok")
	return buf, nil
}

////////////////////////////////////////////////////////////////////////////////
// СТАНЦИЯ 3 — WAV (нормальная, не заглушка)
////////////////////////////////////////////////////////////////////////////////

func (s *ConservativeMediaService) WAV(pcm []byte) []byte {
	log.Printf("[WAV][IN] pcm_len=%d", len(pcm))

	const (
		sampleRate    = 16000
		bitsPerSample = 16
		channels      = 1
	)

	byteRate := sampleRate * channels * (bitsPerSample / 8)
	blockAlign := channels * (bitsPerSample / 8)
	dataSize := len(pcm)
	riffSize := 36 + dataSize

	header := make([]byte, 44)

	copy(header[0:], []byte("RIFF"))
	binary.LittleEndian.PutUint32(header[4:], uint32(riffSize))
	copy(header[8:], []byte("WAVE"))

	copy(header[12:], []byte("fmt "))
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], channels)
	binary.LittleEndian.PutUint32(header[24:], sampleRate)
	binary.LittleEndian.PutUint32(header[28:], uint32(byteRate))
	binary.LittleEndian.PutUint16(header[32:], uint16(blockAlign))
	binary.LittleEndian.PutUint16(header[34:], bitsPerSample)

	copy(header[36:], []byte("data"))
	binary.LittleEndian.PutUint32(header[40:], uint32(dataSize))

	wav := append(header, pcm...)

	log.Printf("[WAV][OUT] wav_len=%d", len(wav))
	return wav
}

////////////////////////////////////////////////////////////////////////////////
// СТАНЦИЯ 4 — STT
////////////////////////////////////////////////////////////////////////////////

func (s *ConservativeMediaService) STT(ctx context.Context, wav []byte) (string, error) {
	log.Printf("[STT][IN] wav_len=%d", len(wav))

	txt, raw, err := s.stt.Recognize(ctx, wav)

	if len(raw) > 0 {
		log.Printf("[STT][RAW YANDEX] %s", raw)
	}

	log.Printf("[STT][OUT] txt=%.40s err=%v", txt, err)

	return txt, err
}

////////////////////////////////////////////////////////////////////////////////
// ОРКЕСТР
////////////////////////////////////////////////////////////////////////////////

func (s *ConservativeMediaService) Process(ctx context.Context, url, roomID string) (*models.Media, error) {

	media, err := s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: url,
		Type:      "audio",
	})
	if err != nil {
		return nil, err
	}

	resolved, err := s.RESOLVE(ctx, url)
	if err != nil {
		return nil, err
	}

	stream, err := s.PCM_START(ctx, resolved)
	if err != nil {
		return nil, err
	}

	go func() {
		defer stream.Close()
		chunk := 1

		for {
			select {
			case <-ctx.Done():
				return

			default:
				pcm, err := s.CUT(stream)
				if err != nil {
					log.Printf("[ORCH][ERR] CUT: %v", err)
					return
				}

				wav := s.WAV(pcm)

				txt, err := s.STT(ctx, wav)
				if err != nil {
					chunk++
					continue
				}

				txt = strings.TrimSpace(txt)
				if txt == "" {
					chunk++
					continue
				}

				s.repo.InsertChunk(ctx, &models.MediaChunk{
					MediaID:     media.ID,
					ChunkNumber: chunk,
					Text:        txt,
				})

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
