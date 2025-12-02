package stations

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
)

// S3PCMtoWAV — station 3: PCM -> WAV (RIFF/WAV header)
type S3PCMtoWAV struct{}

func NewS3PCMtoWAV() *S3PCMtoWAV {
	return &S3PCMtoWAV{}
}

func (s *S3PCMtoWAV) Run(pcm []byte) ([]byte, error) {
	log.Printf("[S3 IN] pcm=%d", len(pcm))

	if len(pcm) == 0 {
		return nil, fmt.Errorf("empty pcm input")
	}

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
	return wav, nil
}
