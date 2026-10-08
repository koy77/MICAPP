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
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OpenAiSpeechClient talks to a Whisper-compatible transcription API
// (OpenAI, Groq) or to Deepgram's native /listen API. Switching providers is
// base URL + model + key, all driven by .env:
//
//	TRANSCRIBE_PROVIDER=openai    → OPENAI_API_KEY    (default model: whisper-1,              url: https://api.openai.com/v1)
//	TRANSCRIBE_PROVIDER=groq      → GROQ_API_KEY      (default model: whisper-large-v3-turbo, url: https://api.groq.com/openai/v1)
//	TRANSCRIBE_PROVIDER=deepgram  → DEEPGRAM_API_KEY  (default model: nova-3,                 url: https://api.deepgram.com/v1)
//
// Optional per-provider overrides: OPENAI_BASE_URL / OPENAI_TRANSCRIBE_MODEL,
// GROQ_BASE_URL / GROQ_TRANSCRIBE_MODEL, DEEPGRAM_BASE_URL / DEEPGRAM_MODEL.
type OpenAiSpeechClient struct {
	apiKey   string
	baseURL  string // e.g. https://api.openai.com/v1 (no trailing slash)
	model    string
	provider string
	client   *http.Client
}

// transcribeProvider describes one selectable transcription backend.
type transcribeProvider struct {
	name        string
	keyVar      string
	baseVar     string
	baseDefault string
	modelVar    string
	modelDef    string
}

var transcribeProviders = map[string]transcribeProvider{
	"openai": {
		name:        "openai",
		keyVar:      "OPENAI_API_KEY",
		baseVar:     "OPENAI_BASE_URL",
		baseDefault: "https://api.openai.com/v1",
		modelVar:    "OPENAI_TRANSCRIBE_MODEL",
		modelDef:    "whisper-1",
	},
	"groq": {
		name:        "groq",
		keyVar:      "GROQ_API_KEY",
		baseVar:     "GROQ_BASE_URL",
		baseDefault: "https://api.groq.com/openai/v1",
		modelVar:    "GROQ_TRANSCRIBE_MODEL",
		modelDef:    "whisper-large-v3-turbo",
	},
	"deepgram": {
		name:        "deepgram",
		keyVar:      "DEEPGRAM_API_KEY",
		baseVar:     "DEEPGRAM_BASE_URL",
		baseDefault: "https://api.deepgram.com/v1",
		modelVar:    "DEEPGRAM_MODEL",
		modelDef:    "nova-3",
	},
}

// TranscriptionResponse represents the JSON response from OpenAI's transcription API
type TranscriptionResponse struct {
	Text string `json:"text"`
}

// NewOpenAiSpeechClient creates the transcription client for the provider
// selected by TRANSCRIBE_PROVIDER (default: openai).
func NewOpenAiSpeechClient() (*OpenAiSpeechClient, error) {
	requested := strings.ToLower(strings.TrimSpace(os.Getenv("TRANSCRIBE_PROVIDER")))
	if requested == "" {
		requested = "openai"
	}

	cfg, ok := transcribeProviders[requested]
	if !ok {
		return nil, fmt.Errorf("unknown TRANSCRIBE_PROVIDER %q (supported: openai, groq)", requested)
	}

	client, err := buildSpeechClient(cfg)
	if err != nil {
		// A missing key on the selected provider must not brick the app: fall
		// back to the other configured provider and log it loudly.
		for name, alt := range transcribeProviders {
			if name == requested {
				continue
			}
			if altClient, altErr := buildSpeechClient(alt); altErr == nil {
				crashf("TRANSCRIBE_PROVIDER=%s не настроен (%v) — fallback на %s (%s)",
					requested, err, altClient.provider, altClient.model)
				client = altClient
				break
			}
		}
		if client == nil {
			return nil, fmt.Errorf("no transcription provider configured: %v", err)
		}
	}

	log.Printf("Transcription provider: %s, model: %s, url: %s, key: %s",
		client.provider, client.model, client.baseURL, client.MaskedKey())
	return client, nil
}

func buildSpeechClient(cfg transcribeProvider) (*OpenAiSpeechClient, error) {
	key := strings.TrimSpace(os.Getenv(cfg.keyVar))
	if key == "" {
		return nil, fmt.Errorf("%s is not set", cfg.keyVar)
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv(cfg.baseVar)), "/")
	if base == "" {
		base = cfg.baseDefault
	}
	model := strings.TrimSpace(os.Getenv(cfg.modelVar))
	if model == "" {
		model = cfg.modelDef
	}
	return &OpenAiSpeechClient{
		apiKey:   key,
		baseURL:  base,
		model:    model,
		provider: cfg.name,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Provider returns the active provider name ("openai" or "groq").
func (c *OpenAiSpeechClient) Provider() string { return c.provider }

// Model returns the active transcription model.
func (c *OpenAiSpeechClient) Model() string { return c.model }

// MaskedKey returns the API key with everything but the first/last characters masked.
func (c *OpenAiSpeechClient) MaskedKey() string {
	k := c.apiKey
	if len(k) <= 8 {
		return "****"
	}
	return k[:4] + "..." + k[len(k)-4:]
}

// Transcribe sends audio data to the configured provider's transcription API.
// Parameters:
//   - audioBytes: audio file data as byte slice (mp3 or wav)
//   - filename: filename (e.g. "recording.mp3") — drives the Content-Type
//   - language: language code (e.g., "ru", "en"; "auto"/"" = auto-detection)
//   - onRequestSent: optional callback called after the request is sent
func (c *OpenAiSpeechClient) Transcribe(audioBytes []byte, filename string, language string, onRequestSent ...func()) (string, error) {
	if c.provider == "deepgram" {
		return c.transcribeDeepgram(audioBytes, filename, language, onRequestSent...)
	}
	return c.transcribeOpenAICompatible(audioBytes, filename, language, onRequestSent...)
}

// transcribeOpenAICompatible posts multipart form data to an OpenAI-compatible
// /audio/transcriptions endpoint (OpenAI, Groq).
func (c *OpenAiSpeechClient) transcribeOpenAICompatible(wavBytes []byte, filename string, language string, onRequestSent ...func()) (string, error) {
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

	// Add model parameter (provider-specific: whisper-1 / whisper-large-v3-turbo)
	err = writer.WriteField("model", c.model)
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
	req, err := http.NewRequest("POST", c.baseURL+"/audio/transcriptions", &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request (this uploads the audio file)
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Call callback after request is sent (now waiting for response/download)
	if len(onRequestSent) > 0 && onRequestSent[0] != nil {
		onRequestSent[0]()
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return "", fmt.Errorf("unauthorized: check your %s API key (%s)", c.provider, c.MaskedKey())
		case http.StatusTooManyRequests:
			return "", fmt.Errorf("rate limit exceeded (%s): please try again later", c.provider)
		case http.StatusBadRequest:
			return "", fmt.Errorf("bad request: %s", string(body))
		default:
			return "", fmt.Errorf("%s API request failed with status %d: %s", c.provider, resp.StatusCode, string(body))
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

// ---------------------------------------------------------------------------
// Deepgram (native API — not OpenAI-compatible)
// ---------------------------------------------------------------------------

// deepgramResponse mirrors the fields we need from POST /v1/listen.
type deepgramResponse struct {
	Results struct {
		Channels []struct {
			Alternatives []struct {
				Transcript string `json:"transcript"`
			} `json:"alternatives"`
		} `json:"channels"`
	} `json:"results"`
	ErrCode string `json:"err_code"`
	ErrMsg  string `json:"err_msg"`
}

// deepgramContentType maps a filename to the Content-Type Deepgram expects for
// raw-body uploads.
func deepgramContentType(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".mp3":
		return "audio/mp3"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	case ".flac":
		return "audio/flac"
	case ".ogg", ".opus":
		return "audio/ogg"
	case ".webm":
		return "audio/webm"
	default:
		return "application/octet-stream"
	}
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// transcribeDeepgram sends audio to Deepgram's pre-recorded /listen endpoint.
// Deepgram is not OpenAI-compatible: raw audio in the body, options in the
// query string, auth header is "Token <key>" (not Bearer), and the transcript
// lives at results.channels[0].alternatives[0].transcript.
func (c *OpenAiSpeechClient) transcribeDeepgram(audioBytes []byte, filename string, language string, onRequestSent ...func()) (string, error) {
	q := url.Values{}
	q.Set("model", c.model)
	q.Set("smart_format", "true") // punctuation, numbers, formatting
	if language != "auto" && language != "" {
		q.Set("language", language)
	} else if strings.HasPrefix(c.model, "nova-3") {
		// nova-3 auto-detection + code-switching across its multilingual set
		q.Set("language", "multi")
	}

	req, err := http.NewRequest("POST", c.baseURL+"/listen?"+q.Encode(), bytes.NewReader(audioBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Token "+c.apiKey)
	req.Header.Set("Content-Type", deepgramContentType(filename))

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Call callback after request is sent (now waiting for response/download)
	if len(onRequestSent) > 0 && onRequestSent[0] != nil {
		onRequestSent[0]()
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr deepgramResponse
		if json.Unmarshal(body, &apiErr) == nil && apiErr.ErrMsg != "" {
			return "", fmt.Errorf("deepgram error %d (%s): %s", resp.StatusCode, apiErr.ErrCode, apiErr.ErrMsg)
		}
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return "", fmt.Errorf("unauthorized: check your deepgram API key (%s)", c.MaskedKey())
		case http.StatusTooManyRequests:
			return "", fmt.Errorf("rate limit exceeded (deepgram): please try again later")
		default:
			return "", fmt.Errorf("deepgram API request failed with status %d: %s", resp.StatusCode, truncateForLog(string(body), 300))
		}
	}

	var parsed deepgramResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse response JSON: %v", err)
	}
	if len(parsed.Results.Channels) == 0 || len(parsed.Results.Channels[0].Alternatives) == 0 {
		return "", fmt.Errorf("deepgram returned no transcript (response: %s)", truncateForLog(string(body), 300))
	}

	return strings.TrimSpace(parsed.Results.Channels[0].Alternatives[0].Transcript), nil
}
