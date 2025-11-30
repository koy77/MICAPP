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
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// OpenAiSpeechClient handles communication with OpenAI's Whisper API
type OpenAiSpeechClient struct {
	apiKey string
	client *http.Client
}

// TranscriptionResponse represents the JSON response from OpenAI's transcription API
type TranscriptionResponse struct {
	Text string `json:"text"`
}

// NewOpenAiSpeechClient creates a new OpenAI speech client
// Reads the API key from the OPENAI_API_KEY environment variable
func NewOpenAiSpeechClient() (*OpenAiSpeechClient, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable is not set")
	}

	return &OpenAiSpeechClient{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Transcribe sends audio data to OpenAI's Whisper API for transcription
// Parameters:
//   - wavBytes: WAV file data as byte slice
//   - filename: Filename for the multipart form (typically "recording.wav")
//   - language: Language code (e.g., "ru" for Russian, "en" for English, "auto" for auto-detection)
//
// Returns:
//   - string: Transcribed text
//   - error: Any error that occurred during the API call
func (c *OpenAiSpeechClient) Transcribe(wavBytes []byte, filename string, language string) (string, error) {
	// Create multipart form data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add the audio file
	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %v", err)
	}

	_, err = fileWriter.Write(wavBytes)
	if err != nil {
		return "", fmt.Errorf("failed to write audio data: %v", err)
	}

	// Add model parameter
	err = writer.WriteField("model", "whisper-1")
	if err != nil {
		return "", fmt.Errorf("failed to write model field: %v", err)
	}

	// Add optional parameters for better transcription
	// Only add language parameter if not "auto" (auto-detection)
	if language != "auto" && language != "" {
		err = writer.WriteField("language", language)
		if err != nil {
			return "", fmt.Errorf("failed to write language field: %v", err)
		}
	}

	err = writer.WriteField("temperature", "0.0") // Use deterministic output
	if err != nil {
		return "", fmt.Errorf("failed to write temperature field: %v", err)
	}

	// Close the writer to finalize the form
	err = writer.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close multipart writer: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/transcriptions", &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return "", fmt.Errorf("unauthorized: check your OpenAI API key")
		case http.StatusTooManyRequests:
			return "", fmt.Errorf("rate limit exceeded: please try again later")
		case http.StatusBadRequest:
			return "", fmt.Errorf("bad request: %s", string(body))
		default:
			return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}
	}

	// Parse JSON response
	var transcriptionResp TranscriptionResponse
	err = json.Unmarshal(body, &transcriptionResp)
	if err != nil {
		return "", fmt.Errorf("failed to parse response JSON: %v", err)
	}

	return transcriptionResp.Text, nil
}
