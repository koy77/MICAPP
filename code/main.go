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
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/go-vgo/robotgo"
	"github.com/gordonklaus/portaudio"
	hook "github.com/robotn/gohook"
)

// copyToClipboard copies text to clipboard using xclip
func copyToClipboard(text string) error {
	cmd := exec.Command("xclip", "-selection", "clipboard")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := stdin.Write([]byte(text)); err != nil {
		return err
	}

	if err := stdin.Close(); err != nil {
		return err
	}

	return cmd.Wait()
}

// clickableStatusLabel is a custom label that handles clicks to copy text
type clickableStatusLabel struct {
	widget.Label
	correctedText *widget.Entry
}

func newClickableStatusLabel(correctedText *widget.Entry) *clickableStatusLabel {
	l := &clickableStatusLabel{
		correctedText: correctedText,
	}
	l.ExtendBaseWidget(l)
	return l
}

func (l *clickableStatusLabel) Tapped(ev *fyne.PointEvent) {
	log.Printf("Status label clicked, copying text to clipboard")
	textToCopy := l.correctedText.Text
	if textToCopy != "" {
		err := copyToClipboard(textToCopy)
		if err != nil {
			log.Printf("Failed to copy text to clipboard: %v", err)
			l.SetText(fmt.Sprintf("Copy failed: %v", err))
		} else {
			log.Printf("Text copied to clipboard")
			l.SetText("Text copied to clipboard")
		}
	} else {
		l.SetText("No text to copy")
	}
}

// setStatusText is a helper function to set text on status label (works with both widget.Label and clickableStatusLabel)
func setStatusText(statusLabel fyne.Widget, text string) {
	if label, ok := statusLabel.(*widget.Label); ok {
		label.SetText(text)
	} else if clickableLabel, ok := statusLabel.(*clickableStatusLabel); ok {
		clickableLabel.SetText(text)
	}
}

// startMouseHook starts monitoring for Ctrl+drag mouse selection using gohook
func (a *AppState) startMouseHook() {
	a.mouseHookMutex.Lock()
	if a.isMouseHookActive {
		a.mouseHookMutex.Unlock()
		return
	}
	a.isMouseHookActive = true
	a.mouseHookMutex.Unlock()

	log.Printf("Mouse hook started - monitoring for Ctrl+Shift+drag selection using gohook (isMouseHookActive=%v, ctrlKeyPressed=%v, isSelecting=%v)",
		a.isMouseHookActive, a.ctrlKeyPressed, a.isSelecting)

	// Start gohook event monitor in separate goroutine
	go a.monitorGohookEvents()
}

// monitorGohookEvents monitors keyboard and mouse events using gohook
func (a *AppState) monitorGohookEvents() {
	log.Printf("Starting gohook event monitor")

	events := hook.Start()
	defer hook.End()

	var lastX, lastY int
	var startX, startY int
	ctrlPressed := false
	shiftPressed := false // Track Shift key state

	log.Printf("Gohook event monitor started, waiting for events...")
	log.Printf("=== KEYBOARD EVENT LOGGING ENABLED - All key presses will be logged ===")
	log.Printf("=== SCREENSHOT CAPTURE: Ctrl + Left Shift + Mouse Drag ===")

	eventCount := 0
	for ev := range events {
		eventCount++

		// Check if we should stop
		a.mouseHookMutex.Lock()
		active := a.isMouseHookActive
		a.mouseHookMutex.Unlock()
		if !active {
			log.Printf("Mouse hook is no longer active, stopping gohook event monitor")
			break
		}

		switch ev.Kind {
		case hook.MouseMove:
			// Update last known mouse position
			lastX = int(ev.X)
			lastY = int(ev.Y)
			// Log first few mouse moves to verify gohook is working
			// (will be noisy, but helps debug)

			// If Ctrl + Shift are both pressed, update selection coordinates
			if ctrlPressed && shiftPressed {
				a.mouseHookMutex.Lock()
				// Update last position while Ctrl is pressed (this is the end point)
				oldX, oldY := a.lastX, a.lastY
				a.lastX, a.lastY = lastX, lastY

				if !a.isSelecting {
					// Mark as selecting
					a.isSelecting = true
					log.Printf("Mouse monitor: Selection started - start=(%d, %d), current=(%d, %d)",
						a.startX, a.startY, lastX, lastY)
				} else if oldX != lastX || oldY != lastY {
					// Only log when position actually changes
					log.Printf("Mouse monitor: Selection updated - start=(%d, %d), current=(%d, %d)",
						a.startX, a.startY, lastX, lastY)
				}
				a.mouseHookMutex.Unlock()
			}

		case hook.KeyDown:
			// Log all keyboard events for debugging - detailed logging to find Function key
			log.Printf("=== KEYDOWN === Rawcode=%d, Keycode=%d, Keychar='%c' (rune=%d), Mask=%d, Button=%d, Clicks=%d, Kind=%d",
				ev.Rawcode, ev.Keycode, ev.Keychar, ev.Keychar, ev.Mask, ev.Button, ev.Clicks, ev.Kind)

			// Check for Ctrl key press
			// Rawcode 65507 is Ctrl in gohook on Linux
			// Keycode 29 is also Ctrl
			// Also check for X11 codes 37 (left Ctrl) and 105 (right Ctrl) for compatibility
			if ev.Rawcode == 65507 || ev.Rawcode == 37 || ev.Rawcode == 105 || ev.Keycode == 29 || ev.Keycode == 37 || ev.Keycode == 105 {
				if !ctrlPressed {
					ctrlPressed = true
					log.Printf("Ctrl key PRESSED (gohook) - Rawcode=%d, Keycode=%d", ev.Rawcode, ev.Keycode)
					// Only start selection if Shift is also pressed
					if shiftPressed {
						// Use last known mouse position as start point, or get current position if not set
						if lastX == 0 && lastY == 0 {
							// Get current mouse position using robotgo
							startX, startY = robotgo.GetMousePos()
							lastX, lastY = startX, startY // Update last position too
						} else {
							startX = lastX
							startY = lastY
						}
						log.Printf("Ctrl+Shift: Starting selection at point: %d, %d", startX, startY)

						a.mouseHookMutex.Lock()
						a.ctrlKeyPressed = true
						a.startX, a.startY = startX, startY
						a.lastX, a.lastY = startX, startY
						a.isSelecting = false // Will be set to true by MouseMove
						log.Printf("Set start position to (%d, %d) when Ctrl+Shift pressed", startX, startY)
						a.mouseHookMutex.Unlock()
					}
				}
			}

			// Check for Left Shift key press
			// Rawcode 50 is Left Shift in X11
			// Keycode 42 is also Left Shift
			if ev.Rawcode == 50 || ev.Keycode == 42 {
				if !shiftPressed {
					shiftPressed = true
					log.Printf("Left Shift key PRESSED (gohook) - Rawcode=%d, Keycode=%d", ev.Rawcode, ev.Keycode)
					// Only start selection if Ctrl is also pressed
					if ctrlPressed {
						// Use last known mouse position as start point, or get current position if not set
						if lastX == 0 && lastY == 0 {
							// Get current mouse position using robotgo
							startX, startY = robotgo.GetMousePos()
							lastX, lastY = startX, startY // Update last position too
						} else {
							startX = lastX
							startY = lastY
						}
						log.Printf("Ctrl+Shift: Starting selection at point: %d, %d", startX, startY)

						a.mouseHookMutex.Lock()
						a.ctrlKeyPressed = true
						a.startX, a.startY = startX, startY
						a.lastX, a.lastY = startX, startY
						a.isSelecting = false // Will be set to true by MouseMove
						log.Printf("Set start position to (%d, %d) when Ctrl+Shift pressed", startX, startY)
						a.mouseHookMutex.Unlock()
					}
				}
			}

		case hook.KeyUp:
			// Log all keyboard events for debugging - detailed logging to find Function key
			log.Printf("=== KEYUP === Rawcode=%d, Keycode=%d, Keychar='%c' (rune=%d), Mask=%d, Button=%d, Clicks=%d, Kind=%d",
				ev.Rawcode, ev.Keycode, ev.Keychar, ev.Keychar, ev.Mask, ev.Button, ev.Clicks, ev.Kind)

			// Check for Ctrl key release
			// Rawcode 65507 is Ctrl in gohook on Linux
			// Keycode 29 is also Ctrl
			// Also check for X11 codes 37 (left Ctrl) and 105 (right Ctrl) for compatibility
			if ev.Rawcode == 65507 || ev.Rawcode == 37 || ev.Rawcode == 105 || ev.Keycode == 29 || ev.Keycode == 37 || ev.Keycode == 105 {
				if ctrlPressed {
					ctrlPressed = false
					log.Printf("Ctrl key RELEASED (gohook) - Rawcode=%d, Keycode=%d", ev.Rawcode, ev.Keycode)
					// Only trigger capture if Shift was also pressed (Ctrl+Shift combination)
					if shiftPressed {
						// Use last known mouse position as end point, or get current position
						endX := lastX
						endY := lastY
						if endX == 0 && endY == 0 {
							// Get current mouse position using robotgo
							endX, endY = robotgo.GetMousePos()
							lastX, lastY = endX, endY // Update last position too
						}
						log.Printf("Ctrl+Shift: Ending selection at point: %d, %d", endX, endY)

						a.mouseHookMutex.Lock()
						a.ctrlKeyPressed = false
						// Update end position
						a.lastX, a.lastY = endX, endY
						log.Printf("Set end position to (%d, %d) when Ctrl+Shift released", endX, endY)
						if a.isSelecting {
							log.Printf("Selection was active, triggering capture")
							// Trigger screenshot capture
							go a.captureSelection()
							a.isSelecting = false
						} else {
							log.Printf("Selection was not active (isSelecting=false), but capturing anyway with start=(%d,%d) end=(%d,%d)",
								a.startX, a.startY, a.lastX, a.lastY)
							// Even if isSelecting is false, we should capture if we have valid coordinates
							if a.startX != 0 || a.startY != 0 || a.lastX != 0 || a.lastY != 0 {
								go a.captureSelection()
							}
						}
						a.mouseHookMutex.Unlock()
					} else {
						// Ctrl released but Shift wasn't pressed, just reset state
						a.mouseHookMutex.Lock()
						a.ctrlKeyPressed = false
						a.mouseHookMutex.Unlock()
					}
				}
			}

			// Check for Left Shift key release
			// Rawcode 50 is Left Shift in X11
			// Keycode 42 is also Left Shift
			if ev.Rawcode == 50 || ev.Keycode == 42 {
				if shiftPressed {
					shiftPressed = false
					log.Printf("Left Shift key RELEASED (gohook) - Rawcode=%d, Keycode=%d", ev.Rawcode, ev.Keycode)
					// Only trigger capture if Ctrl was also pressed (Ctrl+Shift combination)
					if ctrlPressed {
						// Use last known mouse position as end point, or get current position
						endX := lastX
						endY := lastY
						if endX == 0 && endY == 0 {
							// Get current mouse position using robotgo
							endX, endY = robotgo.GetMousePos()
							lastX, lastY = endX, endY // Update last position too
						}
						log.Printf("Ctrl+Shift: Ending selection at point: %d, %d", endX, endY)

						a.mouseHookMutex.Lock()
						a.ctrlKeyPressed = false
						// Update end position
						a.lastX, a.lastY = endX, endY
						log.Printf("Set end position to (%d, %d) when Ctrl+Shift released", endX, endY)
						if a.isSelecting {
							log.Printf("Selection was active, triggering capture")
							// Trigger screenshot capture
							go a.captureSelection()
							a.isSelecting = false
						} else {
							log.Printf("Selection was not active (isSelecting=false), but capturing anyway with start=(%d,%d) end=(%d,%d)",
								a.startX, a.startY, a.lastX, a.lastY)
							// Even if isSelecting is false, we should capture if we have valid coordinates
							if a.startX != 0 || a.startY != 0 || a.lastX != 0 || a.lastY != 0 {
								go a.captureSelection()
							}
						}
						a.mouseHookMutex.Unlock()
					}
				}
			}
		}

		// Small delay to avoid high CPU usage
		time.Sleep(1 * time.Millisecond)
	}

	log.Printf("Gohook event monitor stopped")
}

// stopMouseHook stops the mouse hook monitoring
func (a *AppState) stopMouseHook() {
	log.Printf("Stopping mouse hook (before lock) - isMouseHookActive=%v, ctrlKeyPressed=%v, isSelecting=%v",
		a.isMouseHookActive, a.ctrlKeyPressed, a.isSelecting)
	a.mouseHookMutex.Lock()
	a.isMouseHookActive = false
	a.ctrlKeyPressed = false
	a.isSelecting = false
	a.mouseHookMutex.Unlock()
	log.Printf("Stopping mouse hook (after unlock) - isMouseHookActive=%v, ctrlKeyPressed=%v, isSelecting=%v",
		a.isMouseHookActive, a.ctrlKeyPressed, a.isSelecting)
	// Note: hook.End() is called in monitorGohookEvents defer, which will stop when isMouseHookActive becomes false
}

// CustomTheme provides white text on dark background
type CustomTheme struct {
	fyne.Theme
}

func (t *CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameForeground:
		return color.RGBA{R: 255, G: 255, B: 255, A: 255} // White text
	case theme.ColorNameBackground:
		return color.RGBA{R: 30, G: 30, B: 30, A: 255} // Dark background
	case theme.ColorNameInputBackground:
		return color.RGBA{R: 50, G: 50, B: 50, A: 255} // Darker input background
	case theme.ColorNameInputBorder:
		return color.RGBA{R: 100, G: 100, B: 100, A: 255} // Gray border
	case theme.ColorNameButton:
		return color.RGBA{R: 60, G: 60, B: 60, A: 255} // Dark button background
	case theme.ColorNamePrimary:
		return color.RGBA{R: 255, G: 165, B: 0, A: 255} // Orange for stop button
	default:
		return t.Theme.Color(name, variant)
	}
}

func (t *CustomTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 18 // Larger text size
	default:
		return t.Theme.Size(name)
	}
}

// AppState represents the current state of the application
type AppState struct {
	isRecording        bool
	audioBuffer        []int16
	openaiClient       *OpenAiSpeechClient
	llmClient          *LLMClient
	audioStorage       *AudioStorage
	stream             *portaudio.Stream
	correctedText      *widget.Entry
	recordButton       *widget.Button
	addButton          *widget.Button
	statusLabel        fyne.Widget // Can be *widget.Label or *clickableStatusLabel
	storedAudioList    *widget.List
	lastTranscription  string
	selectedLanguage   string
	recordingMode      string              // "start" or "add"
	activeButton       *widget.Button      // Currently active recording button
	transcriptionQueue []string            // Queue of pending transcriptions
	queueIndicators    []fyne.CanvasObject // Visual indicators for queue
	queueContainer     *fyne.Container     // Container for queue indicators
	imageContainer     *fyne.Container     // Container for image thumbnail
	imageData          []byte              // Raw image data for clipboard
	imageEditorWindow  fyne.Window         // Reference to image editor window (if open)
	mouseHookMutex     sync.Mutex          // Mutex for mouse hook state
	isMouseHookActive  bool                // Whether mouse hook is active
	ctrlKeyPressed     bool                // Whether Ctrl key is currently pressed
	isSelecting        bool                // Whether we're currently selecting a region
	startX, startY     int                 // Selection start coordinates
	lastX, lastY       int                 // Selection end coordinates
	processingMutex    sync.Mutex          // Mutex for processing state
	isProcessing       bool                // Whether audio is being processed
	shouldCancel       bool                // Flag to cancel processing
}

// NewAppState creates a new application state
func NewAppState() (*AppState, error) {
	// Initialize PortAudio
	err := portaudio.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PortAudio: %v", err)
	}

	// Create OpenAI client
	openaiClient, err := NewOpenAiSpeechClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI client: %v", err)
	}

	// Create LLM client for text correction
	llmClient, err := NewLLMClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %v", err)
	}

	// Create audio storage
	audioStorage := NewAudioStorage()

	// Recreate recordings folder on app start
	if err := audioStorage.RecreateRecordingsFolder(); err != nil {
		log.Printf("Warning: Failed to recreate recordings folder: %v", err)
	}

	return &AppState{
		isRecording:        false,
		audioBuffer:        make([]int16, 0),
		openaiClient:       openaiClient,
		llmClient:          llmClient,
		audioStorage:       audioStorage,
		stream:             nil,
		correctedText:      nil,
		recordButton:       nil,
		addButton:          nil,
		statusLabel:        nil,
		storedAudioList:    nil,
		lastTranscription:  "",
		selectedLanguage:   "ru",    // Default to Russian
		recordingMode:      "start", // Default mode
		activeButton:       nil,     // Will be set when recording starts
		transcriptionQueue: make([]string, 0),
		queueIndicators:    make([]fyne.CanvasObject, 0),
		queueContainer:     nil, // Will be set later
		imageContainer:     nil,
		imageData:          nil,
		isMouseHookActive:  false,
		ctrlKeyPressed:     false,
		isSelecting:        false,
		startX:             0,
		startY:             0,
		lastX:              0,
		lastY:              0,
		isProcessing:       false,
		shouldCancel:       false,
	}, nil
}

// Cleanup performs cleanup operations
func (a *AppState) Cleanup() {
	if a.stream != nil {
		a.stream.Close()
	}
	portaudio.Terminate()
}

// StartRecording starts audio recording
func (a *AppState) StartRecording() error {
	// Audio parameters
	sampleRate := 16000.0
	framesPerBuffer := 1024
	numChannels := 1

	// Create audio stream
	stream, err := portaudio.OpenDefaultStream(
		numChannels, 0, // input channels, output channels
		sampleRate, framesPerBuffer, // sample rate, frames per buffer
		a.audioCallback, // callback function
	)
	if err != nil {
		return fmt.Errorf("failed to open audio stream: %v", err)
	}

	a.stream = stream
	a.audioBuffer = make([]int16, 0)

	// Start the stream
	err = stream.Start()
	if err != nil {
		return fmt.Errorf("failed to start audio stream: %v", err)
	}

	a.isRecording = true
	// Only update the active button text and color
	if a.activeButton != nil {
		a.activeButton.SetText("Send")
		// Set orange color for send button using custom theme
		a.activeButton.Importance = widget.HighImportance
	}
	setStatusText(a.statusLabel, "Recording...")

	return nil
}

// StopRecording stops audio recording and processes the audio
func (a *AppState) StopRecording() error {
	if a.stream == nil {
		return fmt.Errorf("no active recording stream")
	}

	// Stop the stream
	err := a.stream.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop audio stream: %v", err)
	}

	err = a.stream.Close()
	if err != nil {
		return fmt.Errorf("failed to close audio stream: %v", err)
	}

	a.stream = nil
	a.isRecording = false

	// Reset cancel flag before processing
	a.processingMutex.Lock()
	a.shouldCancel = false
	a.processingMutex.Unlock()

	// Keep button active (showing "Send") until processing is complete
	// This allows Escape to cancel processing
	log.Printf("StopRecording: keeping button active (activeButton=%v, button text=%s)", a.activeButton != nil, func() string {
		if a.activeButton != nil {
			return a.activeButton.Text
		}
		return "nil"
	}())
	setStatusText(a.statusLabel, "Processing...")

	// Process audio in a goroutine to keep UI responsive
	go a.processAudio()

	return nil
}

// resetActiveButton resets the active button to its original state
func (a *AppState) resetActiveButton() {
	if a.activeButton != nil {
		if a.activeButton == a.recordButton {
			a.activeButton.SetText("Start")
		} else {
			a.activeButton.SetText("Add")
		}
		a.activeButton.Importance = widget.MediumImportance
		a.activeButton = nil
	}
}

// CancelRecording cancels audio recording without processing the audio
func (a *AppState) CancelRecording() error {
	log.Printf("CancelRecording called - isRecording: %v, stream: %v", a.isRecording, a.stream != nil)

	// Set cancel flag to stop any pending transcription
	a.processingMutex.Lock()
	a.shouldCancel = true
	if a.isProcessing {
		a.isProcessing = false
	}
	a.processingMutex.Unlock()

	// Stop and close audio stream
	if a.stream != nil {
		err := a.stream.Stop()
		if err != nil {
			log.Printf("CancelRecording: failed to stop audio stream: %v", err)
			return fmt.Errorf("failed to stop audio stream: %v", err)
		}

		err = a.stream.Close()
		if err != nil {
			log.Printf("CancelRecording: failed to close audio stream: %v", err)
			return fmt.Errorf("failed to close audio stream: %v", err)
		}

		a.stream = nil
	}

	// Reset recording state
	a.isRecording = false
	a.audioBuffer = make([]int16, 0)

	// Remove reserved space for "add" mode
	if a.recordingMode == "add" {
		currentText := a.correctedText.Text
		if len(currentText) >= 2 && currentText[len(currentText)-2:] == "\n\n" {
			currentText = currentText[:len(currentText)-2]
			a.correctedText.SetText(currentText)
		}
	}

	// Reset button and status to original state
	a.resetActiveButton()
	setStatusText(a.statusLabel, "Ready")
	log.Printf("CancelRecording: recording canceled, interface reset to initial state")
	return nil
}

// audioCallback is called by PortAudio for each audio frame
func (a *AppState) audioCallback(in []int16) {
	// Append audio data to buffer
	a.audioBuffer = append(a.audioBuffer, in...)
}

// transcribeWithRetry performs transcription with up to 3 retries
func (a *AppState) transcribeWithRetry(wavData []byte, filename string, language string) (string, error) {
	var lastErr error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Check for cancel before each attempt
		a.processingMutex.Lock()
		shouldCancel := a.shouldCancel
		a.processingMutex.Unlock()
		if shouldCancel {
			log.Printf("transcribeWithRetry: canceled before attempt %d", attempt)
			return "", fmt.Errorf("transcription canceled")
		}

		transcription, err := a.openaiClient.Transcribe(wavData, filename, language)
		if err == nil {
			return transcription, nil
		}

		lastErr = err
		log.Printf("Transcription attempt %d failed: %v", attempt, err)

		if attempt < maxRetries {
			// Check for cancel before retry
			a.processingMutex.Lock()
			shouldCancel = a.shouldCancel
			a.processingMutex.Unlock()
			if shouldCancel {
				log.Printf("transcribeWithRetry: canceled before retry (attempt %d)", attempt+1)
				return "", fmt.Errorf("transcription canceled")
			}
			log.Printf("Retrying transcription (attempt %d/%d)...", attempt+1, maxRetries)
		}
	}

	return "", fmt.Errorf("transcription failed after %d attempts: %v", maxRetries, lastErr)
}

// processAudio processes the recorded audio and sends it to OpenAI asynchronously
func (a *AppState) processAudio() {
	// Set processing flag
	a.processingMutex.Lock()
	a.isProcessing = true
	a.shouldCancel = false
	a.processingMutex.Unlock()

	defer func() {
		a.processingMutex.Lock()
		a.isProcessing = false
		a.shouldCancel = false
		a.processingMutex.Unlock()
	}()

	// Check for cancel before starting
	a.processingMutex.Lock()
	shouldCancel := a.shouldCancel
	a.processingMutex.Unlock()
	if shouldCancel {
		log.Printf("processAudio: canceled before processing")
		setStatusText(a.statusLabel, "Processing canceled")
		a.resetActiveButton()
		return
	}

	if len(a.audioBuffer) == 0 {
		setStatusText(a.statusLabel, "No audio recorded")
		a.resetActiveButton()
		return
	}

	// Check minimum recording duration (3 seconds at 16kHz sample rate)
	minSamples := int(16000 * 3) // 3 seconds at 16kHz
	if len(a.audioBuffer) < minSamples {
		setStatusText(a.statusLabel, "Recording too short (minimum 3 seconds)")

		// If this was an "add" recording, remove the reserved space
		if a.recordingMode == "add" {
			currentText := a.correctedText.Text
			// Remove the last \n\n that we added when starting recording
			if len(currentText) >= 2 && currentText[len(currentText)-2:] == "\n\n" {
				currentText = currentText[:len(currentText)-2]
				a.correctedText.SetText(currentText)
			}
		}
		a.resetActiveButton()
		return
	}

	// Check for cancel before converting
	a.processingMutex.Lock()
	shouldCancel = a.shouldCancel
	a.processingMutex.Unlock()
	if shouldCancel {
		log.Printf("processAudio: canceled before converting audio")
		setStatusText(a.statusLabel, "Processing canceled")
		a.resetActiveButton()
		return
	}

	// Convert int16 samples to bytes
	audioBytes := make([]byte, len(a.audioBuffer)*2)
	for i, sample := range a.audioBuffer {
		// Convert to little-endian bytes
		audioBytes[i*2] = byte(sample & 0xFF)
		audioBytes[i*2+1] = byte((sample >> 8) & 0xFF)
	}

	// Check for cancel before saving recording
	a.processingMutex.Lock()
	shouldCancel = a.shouldCancel
	a.processingMutex.Unlock()
	if shouldCancel {
		log.Printf("processAudio: canceled before saving recording")
		setStatusText(a.statusLabel, "Processing canceled")
		a.resetActiveButton()
		return
	}

	// Save the recording to recordings folder (MP3 128kbps only)
	lastRecording, err := a.audioStorage.SaveLastRecording(audioBytes, 16000)
	if err != nil {
		log.Printf("Failed to save recording: %v", err)
	} else {
		log.Printf("Recording saved as: %s", lastRecording)
	}

	// Check for cancel before adding to queue
	a.processingMutex.Lock()
	shouldCancel = a.shouldCancel
	a.processingMutex.Unlock()
	if shouldCancel {
		log.Printf("processAudio: canceled before adding to transcription queue")
		setStatusText(a.statusLabel, "Processing canceled")
		a.resetActiveButton()
		return
	}

	// Add to transcription queue (asynchronous)
	a.addToQueue(audioBytes, a.recordingMode)
	setStatusText(a.statusLabel, fmt.Sprintf("Processing... (%d in queue)", len(a.transcriptionQueue)))

	// Update stored audio list
	a.updateStoredAudioList()
}

// clearCorrectedText clears the corrected text area
func (a *AppState) clearCorrectedText() {
	a.correctedText.SetText("")
	setStatusText(a.statusLabel, "Ready")
}

// onRecordButtonClick handles the record button click
func (a *AppState) onRecordButtonClick() {
	if !a.isRecording {
		a.recordingMode = "start"       // Set mode to start (replace text)
		a.activeButton = a.recordButton // Set active button
		err := a.StartRecording()
		if err != nil {
			log.Printf("Failed to start recording: %v", err)
			setStatusText(a.statusLabel, fmt.Sprintf("Recording error: %v", err))
		}
	} else {
		err := a.StopRecording()
		if err != nil {
			log.Printf("Failed to stop recording: %v", err)
			setStatusText(a.statusLabel, fmt.Sprintf("Stop error: %v", err))
		}
	}
}

// onAddButtonClick handles the add button click - records and appends text
func (a *AppState) onAddButtonClick() {
	if !a.isRecording {
		a.recordingMode = "add"      // Set mode to add (append text)
		a.activeButton = a.addButton // Set active button

		// Reserve space by adding a new line immediately
		currentText := strings.TrimSpace(a.correctedText.Text)
		if currentText != "" {
			currentText += "\n\n"
		} else {
			currentText = ""
		}
		a.correctedText.SetText(currentText)

		err := a.StartRecording()
		if err != nil {
			log.Printf("Failed to start recording: %v", err)
			setStatusText(a.statusLabel, fmt.Sprintf("Recording error: %v", err))
		}
	} else {
		err := a.StopRecording()
		if err != nil {
			log.Printf("Failed to stop recording: %v", err)
			setStatusText(a.statusLabel, fmt.Sprintf("Stop error: %v", err))
		}
	}
}

// updateQueueIndicators updates the visual queue indicators
func (a *AppState) updateQueueIndicators() {
	if a.queueContainer == nil {
		return
	}

	// Clear existing indicators
	a.queueContainer.RemoveAll()
	a.queueIndicators = make([]fyne.CanvasObject, 0)

	// Create new indicators based on queue length
	for i := 0; i < len(a.transcriptionQueue); i++ {
		// Create an icon that represents data submission/upload
		indicator := widget.NewIcon(theme.UploadIcon())
		indicator.Resize(fyne.NewSize(16, 16))
		a.queueIndicators = append(a.queueIndicators, indicator)
		a.queueContainer.Add(indicator)
	}
}

// addToQueue adds a transcription request to the queue
func (a *AppState) addToQueue(audioData []byte, mode string) {
	// Check if audio data is not empty
	if len(audioData) == 0 {
		setStatusText(a.statusLabel, "No audio data to process")
		return
	}

	// Add to queue
	a.transcriptionQueue = append(a.transcriptionQueue, mode)
	a.updateQueueIndicators()

	// Process asynchronously
	go a.processQueueItem(audioData, mode)
}

// processQueueItem processes a single queue item
func (a *AppState) processQueueItem(audioData []byte, mode string) {
	defer func() {
		// Remove from queue when done
		if len(a.transcriptionQueue) > 0 {
			a.transcriptionQueue = a.transcriptionQueue[1:]
			a.updateQueueIndicators()
		}
		// Reset cancel flag when done (successfully or canceled)
		a.processingMutex.Lock()
		a.shouldCancel = false
		a.processingMutex.Unlock()
		log.Printf("processQueueItem: finished, shouldCancel reset to false")
	}()

	// Check for cancel BEFORE starting transcription
	// If Escape was pressed, we should cancel immediately
	a.processingMutex.Lock()
	shouldCancel := a.shouldCancel
	a.processingMutex.Unlock()
	if shouldCancel {
		log.Printf("processQueueItem: canceled before starting transcription (Escape was pressed)")
		setStatusText(a.statusLabel, "Transcription canceled")
		a.resetActiveButton()
		return
	}

	// Only reset cancel flag AFTER we've confirmed we're starting transcription
	// This allows Escape to work even if pressed right after recording stops
	a.processingMutex.Lock()
	a.shouldCancel = false
	log.Printf("processQueueItem: starting new transcription, shouldCancel reset to false")
	a.processingMutex.Unlock()

	// Convert to MP3 128kbps for transcription (smaller file size, faster upload)
	mp3Data, err := a.audioStorage.ConvertToMP3(audioData, 16000, 128)
	if err != nil {
		log.Printf("Failed to convert to MP3, falling back to WAV: %v", err)
		// Fallback to WAV if MP3 conversion fails
		mp3Data = CreateWAVFile(audioData, 16000, 1)
	}

	// Check for cancel before transcribing
	a.processingMutex.Lock()
	shouldCancel = a.shouldCancel
	a.processingMutex.Unlock()
	if shouldCancel {
		log.Printf("processQueueItem: canceled before transcription")
		setStatusText(a.statusLabel, "Transcription canceled")
		a.resetActiveButton()
		return
	}

	// Transcribe with retry (use selected language)
	language := a.selectedLanguage
	if language == "" {
		language = "ru" // Default to Russian if not set
	}
	log.Printf("Processing transcription with language: %s (using MP3 128kbps)", language)
	transcription, err := a.transcribeWithRetry(mp3Data, "recording.mp3", language)
	if err != nil {
		setStatusText(a.statusLabel, "Transcribed Failed")
		a.resetActiveButton()
		return
	}

	// Check for cancel after transcription
	a.processingMutex.Lock()
	shouldCancel = a.shouldCancel
	a.processingMutex.Unlock()
	if shouldCancel {
		log.Printf("processQueueItem: canceled after transcription")
		setStatusText(a.statusLabel, "Transcription canceled")
		a.resetActiveButton()
		return
	}

	// Update text based on mode
	transcription = strings.TrimSpace(transcription)
	if mode == "add" {
		// Add mode: append to existing text
		// Since we already reserved space with \n\n when recording started,
		// we just need to append the transcription
		currentText := a.correctedText.Text
		currentText += transcription
		a.correctedText.SetText(currentText)

		// Auto-copy to clipboard
		if err := copyToClipboard(currentText); err != nil {
			log.Printf("Failed to copy to clipboard: %v", err)
		} else {
			log.Printf("Text automatically copied to clipboard")
		}
	} else {
		// Start mode: replace text
		a.correctedText.SetText(transcription)

		// Auto-copy to clipboard
		if err := copyToClipboard(transcription); err != nil {
			log.Printf("Failed to copy to clipboard: %v", err)
		} else {
			log.Printf("Text automatically copied to clipboard")
		}
	}

	setStatusText(a.statusLabel, "Transcription completed")

	// Reset button to original state after transcription is complete
	a.resetActiveButton()
	log.Printf("processQueueItem: button reset to initial state after transcription")
}

// updateStoredAudioList updates the stored audio list widget
func (a *AppState) updateStoredAudioList() {
	if a.storedAudioList == nil {
		return
	}

	audioFiles, err := a.audioStorage.GetStoredAudioFiles()
	if err != nil {
		log.Printf("Failed to get stored audio files: %v", err)
		return
	}

	// Create list data
	var listData []string
	for _, file := range audioFiles {
		listItem := fmt.Sprintf("%s (%dkbps, %s)",
			file.Filename,
			file.Bitrate,
			file.Timestamp.Format("15:04:05"))
		listData = append(listData, listItem)
	}

	// Update list widget
	a.storedAudioList.Refresh()
}

func main() {
	// Configure logging to write to app.log file (truncate on each start)
	logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		log.Printf("Failed to open log file: %v, logging to stderr", err)
	} else {
		defer logFile.Close()
		log.SetOutput(logFile)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// Check if OpenAI API key is set
	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Fatal("OPENAI_API_KEY environment variable is not set. Please set it before running the application.")
	}

	// Create application state
	appState, err := NewAppState()
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}
	defer appState.Cleanup()

	// Create Fyne application
	myApp := app.NewWithID("com.voicetranscriber.app")

	// Set custom theme with white text
	myApp.Settings().SetTheme(&CustomTheme{Theme: theme.DarkTheme()})

	// Create main window
	myWindow := myApp.NewWindow("MICAPP")
	myWindow.Resize(fyne.NewSize(300, 700))  // Increased height to accommodate 500px editor + controls
	myWindow.SetFixedSize(false)             // Allow resizing for better UX
	myWindow.SetIcon(resourceRedcubeiconSvg) // Set red cube icon

	// Create UI widgets
	appState.correctedText = widget.NewMultiLineEntry()
	appState.correctedText.SetPlaceHolder("Transcribed and corrected text will appear here...")
	appState.correctedText.TextStyle = fyne.TextStyle{
		Bold: true,
	}
	appState.correctedText.Wrapping = fyne.TextWrapWord
	appState.correctedText.MultiLine = true

	// Use text entry directly
	textContainer := appState.correctedText

	appState.recordButton = widget.NewButton("Start", appState.onRecordButtonClick)
	appState.recordButton.Resize(fyne.NewSize(100, 40))

	appState.addButton = widget.NewButton("Add", appState.onAddButtonClick)
	appState.addButton.Resize(fyne.NewSize(100, 40))

	// Create clickable status label
	statusLabelWidget := newClickableStatusLabel(appState.correctedText)
	statusLabelWidget.SetText("Ready")
	statusLabelWidget.Alignment = fyne.TextAlignCenter
	appState.statusLabel = statusLabelWidget

	// Create stored audio list
	appState.storedAudioList = widget.NewList(
		func() int {
			audioFiles, _ := appState.audioStorage.GetStoredAudioFiles()
			return len(audioFiles)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			audioFiles, _ := appState.audioStorage.GetStoredAudioFiles()
			if id < len(audioFiles) {
				file := audioFiles[id]
				label := obj.(*widget.Label)
				label.SetText(fmt.Sprintf("%s (%dkbps, %s)",
					file.Filename,
					file.Bitrate,
					file.Timestamp.Format("15:04:05")))
			}
		},
	)

	// Create queue indicators container
	queueContainer := container.NewHBox()
	appState.queueContainer = queueContainer // Set reference in AppState

	// Create layout using Border Layout (Method 1)
	buttonContainer := container.NewHBox(
		appState.recordButton,
		appState.addButton,
		widget.NewSeparator(),
		queueContainer,
	)

	// Create image container for captured screenshot thumbnail
	imageContainer := container.NewVBox()
	appState.imageContainer = imageContainer

	// Create status container with image
	statusContainer := container.NewVBox(
		appState.statusLabel,
		widget.NewSeparator(),
		imageContainer,
		widget.NewSeparator(),
	)

	// Create main content using Border Layout
	mainContent := container.NewBorder(
		buttonContainer,                    // Top: controls
		statusContainer,                    // Bottom: status
		nil,                                // Left: none
		nil,                                // Right: none
		container.NewScroll(textContainer), // Center: text editor fills remaining space
	)

	audioTab := container.NewVBox(
		widget.NewLabel("Stored Audio Files"),
		appState.storedAudioList,
	)

	tabs := container.NewAppTabs(
		container.NewTabItem("Text Editor", mainContent),
		container.NewTabItem("Audio Files", audioTab),
	)

	content := tabs

	myWindow.SetContent(content)

	// Add Escape key handler to cancel recording
	myWindow.Canvas().SetOnTypedKey(func(event *fyne.KeyEvent) {
		if event.Name == fyne.KeyEscape {
			log.Printf("=== ESCAPE KEY PRESSED ===")
			log.Printf("ESC key pressed - isRecording: %v", appState.isRecording)

			// Check ONLY isRecording flag - if recording is active, cancel it
			if appState.isRecording {
				log.Printf("ESC: Canceling recording...")
				err := appState.CancelRecording()
				if err != nil {
					log.Printf("ESC: Failed to cancel: %v", err)
					setStatusText(appState.statusLabel, fmt.Sprintf("Cancel error: %v", err))
				} else {
					log.Printf("ESC: Recording canceled, interface reset to initial state")
				}
			} else {
				log.Printf("ESC: No active recording to cancel (isRecording=%v)", appState.isRecording)
			}
		} else if event.Name == fyne.KeyC {
			// Ctrl+C: Copy all text to clipboard using xclip
			textToCopy := appState.correctedText.Text
			if textToCopy != "" {
				err := copyToClipboard(textToCopy)
				if err != nil {
					setStatusText(appState.statusLabel, fmt.Sprintf("Copy failed: %v", err))
				} else {
					setStatusText(appState.statusLabel, "Text copied to clipboard")
				}
			} else {
				setStatusText(appState.statusLabel, "No text to copy")
			}
		}
	})

	// Start mouse hook for Ctrl+drag screenshot capture
	appState.startMouseHook()
	defer appState.stopMouseHook()

	// Set close intercept to close image editor window if open
	myWindow.SetCloseIntercept(func() {
		// Close image editor window if it's open
		if appState.imageEditorWindow != nil {
			log.Printf("Closing image editor window along with main window")
			appState.imageEditorWindow.Close()
			appState.imageEditorWindow = nil
		}
		// Close main window
		myWindow.Close()
	})

	// Show window first
	myWindow.Show()

	// Move window to X=0, Y=200 position (Linux only, using xdotool)
	// This is done after Show() to ensure window is created
	go func() {
		// Small delay to ensure window is fully created
		time.Sleep(200 * time.Millisecond)

		// Try to move window using xdotool
		// First, find the window by name and move it
		cmd := exec.Command("sh", "-c", "xdotool search --name 'MICAPP' | head -1 | xargs -I {} xdotool windowmove {} 0 200")
		if err := cmd.Run(); err != nil {
			// If xdotool fails, try alternative method using wmctrl
			log.Printf("xdotool failed, trying wmctrl: %v", err)
			cmd2 := exec.Command("wmctrl", "-r", "MICAPP", "-e", "0,0,200,-1,-1")
			if err2 := cmd2.Run(); err2 != nil {
				log.Printf("Failed to set window position: %v (xdotool), %v (wmctrl). Window may appear at default position.", err, err2)
			}
		}
	}()

	// Run application
	myApp.Run()
}
