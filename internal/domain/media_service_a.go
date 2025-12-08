package domain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

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

	mu             sync.Mutex
	currentChunkID int
	pending        map[int]string
	mediaID        int
	roomID         string
	events         chan ports.ChunkEvent
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
		repo:    repo,
		s1:      s1,
		s2:      s2,
		s3:      s3,
		s4:      s4,
		s5:      stations.NewS5GPT(gpt),
		pending: map[int]string{},
		events:  make(chan ports.ChunkEvent, 100),
	}
}

func (m *MediaService) Events() <-chan ports.ChunkEvent { return m.events }

// ========================================================================
// PROCESS
// ========================================================================
func (m *MediaService) Process(ctx context.Context,
	srcURL string,
	roomID string,
	mediaID int,
) (*models.Media, error) {

	m.roomID = roomID

	var media *models.Media
	var err error

	if mediaID > 0 {
		media, err = m.repo.GetMediaByID(ctx, mediaID)
		if err != nil {
			return nil, err
		}
		srcURL = media.SourceURL
	} else {
		media, err = m.repo.InsertMedia(ctx, &models.Media{
			SourceURL: srcURL,
			Type:      "audio",
		})
		if err != nil {
			return nil, err
		}
	}
	m.mediaID = media.ID

	last, _ := m.repo.GetLastChunk(ctx, media.ID)
	if last != nil {
		m.currentChunkID = last.ChunkNumber + 1
	} else {
		m.currentChunkID = 1
	}

	go m.ingestLoop(ctx, srcURL)
	return media, nil
}

// ========================================================================
// LOOP
// ========================================================================
func (m *MediaService) ingestLoop(ctx context.Context, srcURL string) {

	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go m.ingestOne(ctx, srcURL)
		}
	}
}

// ========================================================================
// ONE INGEST
// ========================================================================
func (m *MediaService) ingestOne(ctx context.Context, srcURL string) {

	// -------- S1 --------
	audioURL, err := m.s1.Run(ctx, srcURL)
	if err != nil || audioURL == "" {
		println("[S1] fail")
		return
	}
	println("[S1] ok")

	// -------- S2 --------
	pcm, err := m.s2.Run(ctx, audioURL)
	if err != nil || len(pcm) == 0 {
		println("[S2] no pcm")
		return
	}
	println("[S2] ok bytes=", len(pcm))

	chunkID, filePath, err := m.createPendingChunk(ctx, pcm)
	if err != nil {
		println("[INGEST] pending create fail")
		return
	}
	println("[INGEST] pending chunk:", chunkID)

	// WAIT TURN
	for {
		m.mu.Lock()
		ok := (chunkID == m.currentChunkID)
		m.mu.Unlock()
		if ok {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	// -------- S3 --------
	wav := m.s3.Run(pcm)
	println("[S3] wav ok")

	// -------- S4 --------
	raw, err := m.s4.Run(ctx, wav)
	if err != nil {
		println("[S4] err → skip")
		m.advance(chunkID)
		return
	}
	if raw == "" {
		println("[S4] empty → skip")
		m.advance(chunkID)
		return
	}
	println("[S4] text ok")

	// PREV CHUNK
	prevChunk, _ := m.repo.GetLastCompletedChunk(ctx, m.mediaID)
	prevText := ""
	if prevChunk != nil {
		prevText = prevChunk.Text
	}
	println("[S5] prev ok")

	// -------- S5 GPT --------
	println("[GPT] in raw")
	proc, err := m.s5.Run(ctx, prevText, raw)
	if err != nil || proc == "" {
		proc = raw
	}
	println("[GPT] out done")

	// COMPLETE (FIX: передаём mediaID + chunkNumber)
	err = m.repo.CompleteChunk(ctx, m.mediaID, chunkID, proc)
	if err != nil {
		println("[INGEST] save err → advance")
		m.advance(chunkID)
		return
	}

	_ = os.Remove(filePath)

	m.events <- ports.ChunkEvent{
		MediaID:     m.mediaID,
		ChunkNumber: chunkID,
		RoomID:      m.roomID,
		Text:        proc,
	}

	println("[INGEST] done chunk:", chunkID)
	m.advance(chunkID)
}

// ========================================================================
// ADVANCE
// ========================================================================
func (m *MediaService) advance(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentChunkID == id {
		m.currentChunkID++
	}
	delete(m.pending, id)
	println("[ADVANCE] →", m.currentChunkID)
}

// ========================================================================
// CREATE PENDING
// ========================================================================
func (m *MediaService) createPendingChunk(ctx context.Context, pcm []byte) (int, string, error) {

	dir := fmt.Sprintf("/tmp/journalist/media_%d", m.mediaID)
	_ = os.MkdirAll(dir, 0755)

	filename := fmt.Sprintf("chunk_%d.pcm", time.Now().UnixNano())
	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, pcm, 0644); err != nil {
		return 0, "", err
	}

	chunk, err := m.repo.InsertPendingChunk(ctx, m.mediaID, path)
	if err != nil {
		return 0, "", err
	}

	m.mu.Lock()
	if m.currentChunkID == 0 {
		m.currentChunkID = chunk.ChunkNumber
	}
	m.mu.Unlock()

	println("[PENDING] chunk created:", chunk.ChunkNumber)

	return chunk.ChunkNumber, path, nil
}
