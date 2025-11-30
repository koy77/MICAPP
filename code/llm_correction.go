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
	"net/http"
	"os"
	"time"
)

// LLMClient handles communication with OpenAI's GPT API for text correction
type LLMClient struct {
	apiKey string
	client *http.Client
}

// CorrectionRequest represents the request to OpenAI's chat completion API
type CorrectionRequest struct {
	Model          string         `json:"model"`
	Messages       []Message      `json:"messages"`
	MaxTokens      int            `json:"max_tokens"`
	Temperature    float64        `json:"temperature"`
	ResponseFormat ResponseFormat `json:"response_format"`
}

// ResponseFormat defines the JSON response format
type ResponseFormat struct {
	Type string `json:"type"`
}

// Message represents a message in the chat completion request
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CorrectionResponse represents the response from OpenAI's chat completion API
type CorrectionResponse struct {
	Choices []Choice  `json:"choices"`
	Error   *APIError `json:"error,omitempty"`
}

// Choice represents a choice in the response
type Choice struct {
	Message Message `json:"message"`
}

// APIError represents an API error response
type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// CorrectionJSON represents the structured JSON response for text correction
type CorrectionJSON struct {
	OriginalText  string   `json:"original_text"`
	CorrectedText string   `json:"corrected_text"`
	Changes       []Change `json:"changes"`
	Confidence    float64  `json:"confidence"`
}

// Change represents a specific change made to the text
type Change struct {
	Type        string `json:"type"` // "grammar", "punctuation", "capitalization", "clarity"
	Original    string `json:"original"`
	Corrected   string `json:"corrected"`
	Description string `json:"description"`
}

// NewLLMClient creates a new LLM client for text correction
func NewLLMClient() (*LLMClient, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable is not set")
	}

	return &LLMClient{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// CorrectText sends transcribed text to OpenAI's GPT API for correction and improvement
func (c *LLMClient) CorrectText(transcribedText string) (string, error) {
	// Create the correction prompt with JSON format specification
	prompt := fmt.Sprintf(`Please correct and improve the following transcribed text. Fix any grammar errors, punctuation, capitalization, and make it more readable while preserving the original meaning.

Return your response in the following JSON format:
{
  "original_text": "the original transcribed text",
  "corrected_text": "the corrected and improved text",
  "changes": [
    {
      "type": "grammar|punctuation|capitalization|clarity",
      "original": "original phrase",
      "corrected": "corrected phrase",
      "description": "brief description of the change"
    }
  ],
  "confidence": 0.95
}

Original text: "%s"`, transcribedText)

	// Create the request with JSON response format
	request := CorrectionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   1000,
		Temperature: 0.3, // Lower temperature for more consistent corrections
		ResponseFormat: ResponseFormat{
			Type: "json_object",
		},
	}

	// Convert to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

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
	var correctionResp CorrectionResponse
	err = json.Unmarshal(body, &correctionResp)
	if err != nil {
		return "", fmt.Errorf("failed to parse response JSON: %v", err)
	}

	// Check for API errors
	if correctionResp.Error != nil {
		return "", fmt.Errorf("API error: %s", correctionResp.Error.Message)
	}

	// Extract corrected text
	if len(correctionResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices received")
	}

	// Parse the JSON content from the response
	var correctionJSON CorrectionJSON
	err = json.Unmarshal([]byte(correctionResp.Choices[0].Message.Content), &correctionJSON)
	if err != nil {
		// Fallback: if JSON parsing fails, return the raw content
		log.Printf("Failed to parse correction JSON, using raw content: %v", err)
		return correctionResp.Choices[0].Message.Content, nil
	}

	// Log the changes made for debugging
	if len(correctionJSON.Changes) > 0 {
		log.Printf("Applied %d corrections with confidence %.2f", len(correctionJSON.Changes), correctionJSON.Confidence)
		for _, change := range correctionJSON.Changes {
			log.Printf("  %s: '%s' -> '%s' (%s)", change.Type, change.Original, change.Corrected, change.Description)
		}
	}

	return correctionJSON.CorrectedText, nil
}

// CorrectTextWithContext sends transcribed text with context for better correction
func (c *LLMClient) CorrectTextWithContext(transcribedText string, context string) (string, error) {
	// Create the correction prompt with context and JSON format specification
	prompt := fmt.Sprintf(`Please correct and improve the following transcribed text. Use the provided context to better understand the intended meaning. Fix any grammar errors, punctuation, capitalization, and make it more readable while preserving the original meaning.

Return your response in the following JSON format:
{
  "original_text": "the original transcribed text",
  "corrected_text": "the corrected and improved text",
  "changes": [
    {
      "type": "grammar|punctuation|capitalization|clarity",
      "original": "original phrase",
      "corrected": "corrected phrase",
      "description": "brief description of the change"
    }
  ],
  "confidence": 0.95
}

Context: %s

Original text: "%s"`, context, transcribedText)

	// Create the request with JSON response format
	request := CorrectionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   1000,
		Temperature: 0.3,
		ResponseFormat: ResponseFormat{
			Type: "json_object",
		},
	}

	// Convert to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

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
	var correctionResp CorrectionResponse
	err = json.Unmarshal(body, &correctionResp)
	if err != nil {
		return "", fmt.Errorf("failed to parse response JSON: %v", err)
	}

	// Check for API errors
	if correctionResp.Error != nil {
		return "", fmt.Errorf("API error: %s", correctionResp.Error.Message)
	}

	// Extract corrected text
	if len(correctionResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices received")
	}

	// Parse the JSON content from the response
	var correctionJSON CorrectionJSON
	err = json.Unmarshal([]byte(correctionResp.Choices[0].Message.Content), &correctionJSON)
	if err != nil {
		// Fallback: if JSON parsing fails, return the raw content
		log.Printf("Failed to parse correction JSON, using raw content: %v", err)
		return correctionResp.Choices[0].Message.Content, nil
	}

	// Log the changes made for debugging
	if len(correctionJSON.Changes) > 0 {
		log.Printf("Applied %d corrections with confidence %.2f", len(correctionJSON.Changes), correctionJSON.Confidence)
		for _, change := range correctionJSON.Changes {
			log.Printf("  %s: '%s' -> '%s' (%s)", change.Type, change.Original, change.Corrected, change.Description)
		}
	}

	return correctionJSON.CorrectedText, nil
}

// CorrectTextDetailed returns the full JSON correction response with detailed changes
func (c *LLMClient) CorrectTextDetailed(transcribedText string) (*CorrectionJSON, error) {
	// Create the correction prompt with JSON format specification
	prompt := fmt.Sprintf(`Please correct and improve the following transcribed text. Fix any grammar errors, punctuation, capitalization, and make it more readable while preserving the original meaning.

Return your response in the following JSON format:
{
  "original_text": "the original transcribed text",
  "corrected_text": "the corrected and improved text",
  "changes": [
    {
      "type": "grammar|punctuation|capitalization|clarity",
      "original": "original phrase",
      "corrected": "corrected phrase",
      "description": "brief description of the change"
    }
  ],
  "confidence": 0.95
}

Original text: "%s"`, transcribedText)

	// Create the request with JSON response format
	request := CorrectionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   1000,
		Temperature: 0.3,
		ResponseFormat: ResponseFormat{
			Type: "json_object",
		},
	}

	// Convert to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return nil, fmt.Errorf("unauthorized: check your OpenAI API key")
		case http.StatusTooManyRequests:
			return nil, fmt.Errorf("rate limit exceeded: please try again later")
		case http.StatusBadRequest:
			return nil, fmt.Errorf("bad request: %s", string(body))
		default:
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}
	}

	// Parse JSON response
	var correctionResp CorrectionResponse
	err = json.Unmarshal(body, &correctionResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %v", err)
	}

	// Check for API errors
	if correctionResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", correctionResp.Error.Message)
	}

	// Extract corrected text
	if len(correctionResp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices received")
	}

	// Parse the JSON content from the response
	var correctionJSON CorrectionJSON
	err = json.Unmarshal([]byte(correctionResp.Choices[0].Message.Content), &correctionJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse correction JSON: %v", err)
	}

	return &correctionJSON, nil
}
