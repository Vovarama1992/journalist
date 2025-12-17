package stations

import (
	"bytes"
	"encoding/binary"
)

type S3PCMtoWAV struct{}

func NewS3PCMtoWAV() *S3PCMtoWAV { return &S3PCMtoWAV{} }

func (s *S3PCMtoWAV) Run(pcm []byte) []byte {

	println("[S3] start")

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

	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")

	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(channels))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	_ = binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	_ = binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))

	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	_, _ = buf.Write(pcm)

	wav := buf.Bytes()

	println("[S3] ok")
	return wav
}
