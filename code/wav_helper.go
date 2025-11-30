// MIT License
// Copyright (c) 2024 VoiceTranscriber
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

package main

import (
	"bytes"
	"encoding/binary"
)

// WAVHeader represents the structure of a WAV file header
type WAVHeader struct {
	RiffHeader    [4]byte // "RIFF"
	FileSize      uint32  // File size - 8
	WaveHeader    [4]byte // "WAVE"
	FmtHeader     [4]byte // "fmt "
	FmtChunkSize  uint32  // Format chunk size (16 for PCM)
	AudioFormat   uint16  // Audio format (1 for PCM)
	NumChannels   uint16  // Number of channels
	SampleRate    uint32  // Sample rate
	ByteRate      uint32  // Byte rate
	BlockAlign    uint16  // Block align
	BitsPerSample uint16  // Bits per sample
	DataHeader    [4]byte // "data"
	DataSize      uint32  // Data size
}

// CreateWAVFile creates a WAV file from raw PCM audio data
// Parameters:
//   - pcmData: Raw PCM audio data (16-bit samples)
//   - sampleRate: Sample rate in Hz (e.g., 16000)
//   - numChannels: Number of audio channels (1 for mono, 2 for stereo)
//
// Returns:
//   - []byte: Complete WAV file as byte slice
func CreateWAVFile(pcmData []byte, sampleRate uint32, numChannels uint16) []byte {
	dataSize := uint32(len(pcmData))
	fileSize := uint32(36 + dataSize) // 36 bytes for header + data size

	// Calculate derived values
	bitsPerSample := uint16(16) // 16-bit samples
	byteRate := sampleRate * uint32(numChannels) * uint32(bitsPerSample) / 8
	blockAlign := numChannels * bitsPerSample / 8

	// Create WAV header
	header := WAVHeader{
		RiffHeader:    [4]byte{'R', 'I', 'F', 'F'},
		FileSize:      fileSize,
		WaveHeader:    [4]byte{'W', 'A', 'V', 'E'},
		FmtHeader:     [4]byte{'f', 'm', 't', ' '},
		FmtChunkSize:  16, // Standard PCM format chunk size
		AudioFormat:   1,  // PCM format
		NumChannels:   numChannels,
		SampleRate:    sampleRate,
		ByteRate:      byteRate,
		BlockAlign:    blockAlign,
		BitsPerSample: bitsPerSample,
		DataHeader:    [4]byte{'d', 'a', 't', 'a'},
		DataSize:      dataSize,
	}

	// Write header to buffer
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, header)

	// Append PCM data
	buf.Write(pcmData)

	return buf.Bytes()
}
