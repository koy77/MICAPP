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
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// AudioStorage manages storage of audio files with different bitrates
type AudioStorage struct {
	baseDir string
}

// AudioFile represents a stored audio file with metadata
type AudioFile struct {
	Filename   string
	Bitrate    int
	SampleRate int
	Duration   time.Duration
	Timestamp  time.Time
	Size       int64
}

// NewAudioStorage creates a new audio storage manager
func NewAudioStorage() *AudioStorage {
	// Use recordings folder in the current directory
	baseDir := filepath.Join(".", "recordings")

	// Create directory if it doesn't exist
	os.MkdirAll(baseDir, 0755)

	return &AudioStorage{
		baseDir: baseDir,
	}
}

// RecreateRecordingsFolder removes and recreates the recordings folder
func (as *AudioStorage) RecreateRecordingsFolder() error {
	// Remove the entire recordings folder if it exists
	if err := os.RemoveAll(as.baseDir); err != nil {
		log.Printf("Failed to remove recordings folder: %v", err)
		return err
	}

	// Create the folder again
	if err := os.MkdirAll(as.baseDir, 0755); err != nil {
		log.Printf("Failed to create recordings folder: %v", err)
		return err
	}

	log.Printf("Recordings folder recreated successfully")
	return nil
}

// StoreAudio stores audio data as MP3 with different bitrates
func (as *AudioStorage) StoreAudio(pcmData []byte, sampleRate uint32) ([]AudioFile, error) {
	timestamp := time.Now()
	var storedFiles []AudioFile

	// Define different bitrates to store
	bitrates := []int{128, 192, 256, 320} // kbps

	for _, bitrate := range bitrates {
		// Create filename with timestamp and bitrate
		filename := fmt.Sprintf("recording_%s_%dkbps.mp3",
			timestamp.Format("20060102_150405"), bitrate)
		filepath := filepath.Join(as.baseDir, filename)

		// Convert PCM to MP3 using ffmpeg
		mp3Data, err := as.convertPCMToMP3(pcmData, sampleRate, bitrate)
		if err != nil {
			log.Printf("Failed to convert PCM to MP3 at %dkbps: %v (skipping this bitrate)", bitrate, err)
			continue // Skip this bitrate if conversion fails
		}

		// Write MP3 file
		err = os.WriteFile(filepath, mp3Data, 0644)
		if err != nil {
			log.Printf("Failed to write MP3 file at %dkbps: %v (skipping this bitrate)", bitrate, err)
			continue // Skip this bitrate if write fails
		}

		// Get file info
		fileInfo, err := os.Stat(filepath)
		if err != nil {
			log.Printf("Failed to stat MP3 file at %dkbps: %v (skipping this bitrate)", bitrate, err)
			continue
		}

		// Calculate duration (approximate)
		duration := time.Duration(len(pcmData)) * time.Second / time.Duration(sampleRate*2) // 2 bytes per sample

		storedFiles = append(storedFiles, AudioFile{
			Filename:   filename,
			Bitrate:    bitrate,
			SampleRate: int(sampleRate),
			Duration:   duration,
			Timestamp:  timestamp,
			Size:       fileInfo.Size(),
		})
	}

	return storedFiles, nil
}

// SaveLastRecording saves the recording as MP3 128kbps to the recordings folder
func (as *AudioStorage) SaveLastRecording(pcmData []byte, sampleRate uint32) (string, error) {
	timestamp := time.Now()
	baseFilename := fmt.Sprintf("recording_%s", timestamp.Format("20060102_150405"))

	// Save only MP3 128kbps (used for transcription)
	mp3Filename := baseFilename + ".mp3"
	mp3Filepath := filepath.Join(as.baseDir, mp3Filename)

	mp3Data, err := as.convertPCMToMP3(pcmData, sampleRate, 128)
	if err != nil {
		return "", fmt.Errorf("failed to convert to MP3: %v", err)
	}

	err = os.WriteFile(mp3Filepath, mp3Data, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write MP3 file: %v", err)
	}

	// Verify MP3 file was written correctly
	mp3Info, err := os.Stat(mp3Filepath)
	if err != nil {
		log.Printf("Failed to stat MP3 file: %v", err)
	} else {
		log.Printf("MP3 file saved: %s (size: %d bytes, bitrate: 128 kbps)", mp3Filepath, mp3Info.Size())
	}

	return mp3Filename, nil
}

// ConvertToMP3 converts PCM data to MP3 format using ffmpeg (public method)
func (as *AudioStorage) ConvertToMP3(pcmData []byte, sampleRate uint32, bitrate int) ([]byte, error) {
	return as.convertPCMToMP3(pcmData, sampleRate, bitrate)
}

// convertPCMToMP3 converts PCM data to MP3 format using ffmpeg
func (as *AudioStorage) convertPCMToMP3(pcmData []byte, sampleRate uint32, bitrate int) ([]byte, error) {
	// First, create a temporary WAV file from PCM data
	wavData := CreateWAVFile(pcmData, sampleRate, 1)

	// Create temporary files for input (WAV) and output (MP3)
	tmpWavFile, err := os.CreateTemp("", "temp_*.wav")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp WAV file: %v", err)
	}
	defer os.Remove(tmpWavFile.Name())
	defer tmpWavFile.Close()

	tmpMp3File, err := os.CreateTemp("", "temp_*.mp3")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp MP3 file: %v", err)
	}
	defer os.Remove(tmpMp3File.Name())
	defer tmpMp3File.Close()

	// Write WAV data to temp file
	if _, err := tmpWavFile.Write(wavData); err != nil {
		return nil, fmt.Errorf("failed to write WAV data: %v", err)
	}
	tmpWavFile.Close()

	// Use ffmpeg to convert WAV to MP3
	cmd := exec.Command("ffmpeg",
		"-i", tmpWavFile.Name(),
		"-codec:a", "libmp3lame",
		"-b:a", fmt.Sprintf("%dk", bitrate),
		"-y", // Overwrite output file
		tmpMp3File.Name(),
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// If ffmpeg is not available, return error
		log.Printf("ffmpeg conversion failed: %v, stderr: %s", err, stderr.String())
		return nil, fmt.Errorf("ffmpeg conversion failed: %v (ffmpeg may not be installed)", err)
	}

	// Read the MP3 file
	mp3Data, err := os.ReadFile(tmpMp3File.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read MP3 file: %v", err)
	}

	return mp3Data, nil
}

// GetStoredAudioFiles returns all stored audio files
func (as *AudioStorage) GetStoredAudioFiles() ([]AudioFile, error) {
	files, err := os.ReadDir(as.baseDir)
	if err != nil {
		return nil, err
	}

	var audioFiles []AudioFile
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".mp3" {
			fileInfo, err := file.Info()
			if err != nil {
				continue
			}

			// Parse filename to extract metadata
			audioFile := AudioFile{
				Filename:  file.Name(),
				Timestamp: fileInfo.ModTime(),
				Size:      fileInfo.Size(),
			}

			// Extract bitrate from filename (simplified)
			if len(file.Name()) > 10 {
				// Assume format: recording_YYYYMMDD_HHMMSS_XXXkbps.mp3
				audioFile.Bitrate = 128 // Default, would parse from filename in real implementation
			}

			audioFiles = append(audioFiles, audioFile)
		}
	}

	return audioFiles, nil
}

// DeleteAudioFile deletes a stored audio file
func (as *AudioStorage) DeleteAudioFile(filename string) error {
	filepath := filepath.Join(as.baseDir, filename)
	return os.Remove(filepath)
}

// GetAudioFilePath returns the full path to an audio file
func (as *AudioStorage) GetAudioFilePath(filename string) string {
	return filepath.Join(as.baseDir, filename)
}
