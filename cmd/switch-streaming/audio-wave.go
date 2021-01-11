package main

import (
	"bytes"
	"encoding/binary"
	"io"
)

type WaveHeader struct {
	ChunkID       [4]byte
	ChunkSize     uint32
	Format        [4]byte
	SubChunk1ID   [4]byte
	SubChunk1Size uint32
	AudioFormat   uint16
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	SampleBits    uint16
	SubChunk2ID   [4]byte
	SubChunk2Size uint32
}

func (h *WaveHeader) Write(w io.Writer) (int64, error) {
	return int64(binary.Size(h)), binary.Write(w, binary.LittleEndian, h)
}
func (h *WaveHeader) Bytes() []byte {
	var buff bytes.Buffer
	h.Write(&buff)
	return buff.Bytes()
}

func NewWaveHeader(dataSize uint32, channelCount uint16, sampleRate uint32, sampleBits uint16) *WaveHeader {
	h := WaveHeader{}
	copy(h.ChunkID[:], []byte("RIFF"))
	h.ChunkSize = dataSize + 44 - 8
	copy(h.Format[:], []byte("WAVE"))
	copy(h.SubChunk1ID[:], []byte("fmt "))
	h.SubChunk1Size = 16
	h.AudioFormat = 1
	h.NumChannels = channelCount
	h.SampleRate = sampleRate
	h.ByteRate = uint32(channelCount) * sampleRate * uint32(sampleBits/8)
	h.BlockAlign = channelCount * sampleBits / 8
	h.SampleBits = sampleBits
	copy(h.SubChunk2ID[:], []byte("data"))
	h.SubChunk2Size = dataSize
	return &h
}
