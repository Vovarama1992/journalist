package domain

import (
	"context"
	"fmt"
	"log"
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
	pending        map[int]string // chunkID → filePath
	mediaID        int
	roomID         string
	events         chan ports.ChunkEvent
}

func NewMediaService(repo ports.MediaRepository,
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
// START PROCESS
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
// IN GEST LOOP — запускаем ingestOne каждые 8 секунд
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
// ONE INGEST — снимает 15 сек PCM, создаёт pending chunk
// ========================================================================
func (m *MediaService) ingestOne(ctx context.Context, srcURL string) {

	// -------- S1 --------
	audioURL, err := m.s1.Run(ctx, srcURL)
	if err != nil {
		log.Printf("[INGEST] S1 err=%v", err)
		return
	}

	// -------- S2 --------
	pcm, err := m.s2.Run(ctx, audioURL)
	if err != nil || len(pcm) == 0 {
		log.Printf("[INGEST] S2 err=%v", err)
		return
	}

	chunkID, filePath, err := m.createPendingChunk(ctx, pcm)
	if err != nil {
		log.Printf("[INGEST] pending create err=%v", err)
		return
	}

	// =====================================================================
	// WAIT FOR TURN — блок до тех пор, пока chunkID != currentChunkID
	// =====================================================================
	for {
		m.mu.Lock()
		ok := (chunkID == m.currentChunkID)
		m.mu.Unlock()

		if ok {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	// =====================================================================
	// PROCESS CURRENT CHUNK — S3 / S4 / S5
	// =====================================================================
	wav := m.s3.Run(pcm)

	raw, err := m.s4.Run(ctx, wav)
	if err != nil {
		log.Printf("[INGEST] S4 err=%v → SKIP but advance current", err)
		m.advance(chunkID)
		return
	}
	if raw == "" {
		log.Printf("[INGEST] S4 empty → SKIP but advance current")
		m.advance(chunkID)
		return
	}

	prevChunk, _ := m.repo.GetLastCompletedChunk(ctx, m.mediaID)
	prevText := ""
	if prevChunk != nil {
		log.Printf("[S5][PREV_CHUNK] id=%d num=%d text=%q",
			prevChunk.ID,
			prevChunk.ChunkNumber,
			prevChunk.Text,
		)
		prevText = prevChunk.Text
	} else {
		log.Printf("[S5][PREV_CHUNK] none")
	}

	proc, err := m.s5.Run(ctx, prevText, raw)
	if err != nil || proc == "" {
		proc = raw
	}

	// =====================================================================
	// COMPLETE CHUNK
	// =====================================================================
	err = m.repo.CompleteChunk(ctx, chunkID, proc)
	if err != nil {
		log.Printf("[INGEST] save err=%v → advance current", err)
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

	m.advance(chunkID)
}

// ========================================================================
// ADVANCE — всегда продвигает указатель currentChunkID
// ========================================================================
func (m *MediaService) advance(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentChunkID == id {
		m.currentChunkID++
	}
	delete(m.pending, id)
}

// ========================================================================
// CREATE PENDING CHUNK
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
		// значит это самый первый pending
		m.currentChunkID = chunk.ChunkNumber
	}
	m.mu.Unlock()

	return chunk.ChunkNumber, path, nil
}
