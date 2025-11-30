You are an expert Go developer. Generate a minimal but production-quality desktop application called "VoiceTranscriber" for Ubuntu Linux with the following requirements:

GOAL
- Simple window with:
  - A TextArea (multi-line, read-only) that shows transcription results (and accumulates previous transcriptions).
  - A "Start" (Play/Record) button that starts recording microphone input and, when clicked again, stops recording and sends the recorded audio to OpenAI Speech-to-Text (Whisper) API for transcription.
  - A "+" (Plus) button that clears the TextArea and resets session state.
  - Visual recording indicator (e.g., red dot or label "Recording...") when active.
- Recording should be saved to a WAV buffer (PCM 16-bit, 16k or 16/44.1k sample rate — choose 16k or 16k/16000 recommended for speech).
- After transcription result returns, append it (with timestamp) to the TextArea.

ARCHITECTURE & TECH DETAILS
- Project: Go 1.19+ with Fyne GUI framework for Ubuntu Linux. Single-window app.
- Use github.com/gordonklaus/portaudio for microphone capture — add go mod dependency.
- Implement recording into memory buffer and then into a .wav byte slice (ensure proper WAV header).
- Use net/http to send a multipart/form-data POST to OpenAI audio transcription endpoint /v1/audio/transcriptions (model whisper-1). Use OPENAI_API_KEY from environment variable (do not hardcode).
- Use goroutines for audio recording to keep UI responsive.
- Add proper error handling and user feedback (errors shown in TextArea or dialog).
- Keep code simple and well-commented; split out a small OpenAiSpeechClient helper struct that accepts byte slice wav and returns transcription string.
- Provide main.go, openai_client.go, and wav_helper.go (helper to write WAV header to memory buffer).
- Include README instructions: how to set OPENAI_API_KEY env var, install Go dependencies, and how to build/run on Ubuntu.
- Use MIT-style permissive license header comment (short).

IMPLEMENTATION DETAILS (what to generate)
1. main.go:
   - Fyne window with TextArea widget (name="transcriptionText"), Button (name="recordButton", text="Start"), Button (name="plusButton", text="+"), Label for status (name="statusLabel").
2. main.go (continued):
   - Use portaudio.OpenDefaultStream() to capture PCM 16-bit, 16000 Hz.
   - On recordButton click:
     - If not recording: start capture in separate goroutine, change UI to "Recording..." (red), change button text to "Stop".
     - If recording: stop capture, convert captured bytes to WAV, call OpenAiSpeechClient.Transcribe(...), append with timestamp to transcriptionText, set UI back to idle.
   - plusButton click: clear transcriptionText.
   - Use goroutines for audio recording to keep UI responsive.
3. openai_client.go:
   - Constructor reads API key from environment variable OPENAI_API_KEY.
   - func (c *OpenAiSpeechClient) Transcribe(wavBytes []byte, filename string) (string, error):
     - Build multipart/form-data request:
       - Add file with name "file" and filename "recording.wav".
       - Add "whisper-1" for model parameter.
       - Optionally add language or temperature if desired.
     - POST to https://api.openai.com/v1/audio/transcriptions.
     - Parse JSON response and return text field.
     - Consider HTTP 401/429 handling and surface readable error messages.
   - Use Authorization header Bearer {API_KEY} where API_KEY read from env OPENAI_API_KEY.
4. wav_helper.go:
   - Provide function to wrap raw PCM 16-bit samples into WAV header (RIFF) and return []byte.
   - Ensure correct byte ordering and format chunk.

SECURITY & BEST PRACTICES
- Read API key from environment variable OPENAI_API_KEY or from a secure local config (do not embed in repo).
- Add user-friendly README steps for environment variable setup on Ubuntu (bash examples).
- Mention that the OpenAI Speech-to-Text endpoint needs multipart/form-data and may accept other params; link to official docs.

UBUNTU-SPECIFIC REQUIREMENTS
- Ensure the application works with Ubuntu's audio system (ALSA/PulseAudio).
- Include instructions for installing Go 1.19+ on Ubuntu:
  - `sudo apt update && sudo apt install -y golang-go`
- Include instructions for setting up environment variables in Ubuntu:
  - `export OPENAI_API_KEY="your-api-key-here"` (for current session)
  - Add to `~/.bashrc` or `~/.profile` for persistence: `echo 'export OPENAI_API_KEY="your-api-key-here"' >> ~/.bashrc`
- Include instructions for audio permissions and microphone access on Ubuntu.
- Test with Ubuntu's default audio drivers and PulseAudio.
- Consider Ubuntu-specific audio device enumeration and selection.
- Install required Go dependencies: `go mod tidy` (will install fyne.io/fyne/v2 and github.com/gordonklaus/portaudio)

UBUNTU SETUP INSTRUCTIONS
1. Install Go and dependencies:
   ```bash
   sudo apt update
   sudo apt install -y golang-go
   ```

2. Install audio dependencies:
   ```bash
   sudo apt install -y libasound2-dev libpulse-dev portaudio19-dev
   ```

3. Initialize Go module and install dependencies:
   ```bash
   go mod init voicetranscriber
   go mod tidy
   ```

4. Set up OpenAI API key:
   ```bash
   # For current session
   export OPENAI_API_KEY="your-api-key-here"
   
   # For persistence (add to ~/.bashrc)
   echo 'export OPENAI_API_KEY="your-api-key-here"' >> ~/.bashrc
   source ~/.bashrc
   ```

5. Build and run the application:
   ```bash
   go build -o voicetranscriber
   ./voicetranscriber
   ```

6. Audio permissions:
   - Ensure microphone access is granted in Ubuntu settings
   - Test audio input with: `arecord -l` to list audio devices
   - Verify PulseAudio is running: `pulseaudio --check -v`