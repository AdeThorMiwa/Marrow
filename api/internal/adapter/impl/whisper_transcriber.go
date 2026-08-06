package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"

	api "marrow/internal/adapter/api"
	model "marrow/internal/model"
)

const whisperModelName = "whisper-medium"

type WhisperCppTranscriber struct {
	BaseURL string
	Client  *http.Client
}

// NewWhisperCppTranscriber asserts a live whisper.cpp server is actually
// reachable at baseURL before returning — same rationale as
// NewOllamaEmbedder: fail loudly at startup, not on the first real
// transcription job.
func NewWhisperCppTranscriber(baseURL string) *WhisperCppTranscriber {
	assertServiceUp("whisper.cpp", baseURL)
	return &WhisperCppTranscriber{
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 5 * time.Minute}, // transcription can take a while for long-form audio/video
	}
}

type whisperTranscriptionResponse struct {
	Text string `json:"text"`
}

// Transcribe takes raw media bytes directly — no fetching, no knowledge of
// where media.Buffer came from. MediaResolver (adapter/api/media.go) is
// responsible for producing those bytes before this is ever called.
func (w *WhisperCppTranscriber) Transcribe(ctx context.Context, media model.Media) (*api.TranscriptionResponse, error) {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile("file", "audio")
	if err != nil {
		return nil, fmt.Errorf("failed to build multipart request: %w", err)
	}
	if _, err := part.Write(media.Buffer); err != nil {
		return nil, fmt.Errorf("failed to write media into request: %w", err)
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize multipart request: %w", err)
	}

	// whisper.cpp's server (whisper-server) does not expose an
	// OpenAI-compatible /v1/audio/transcriptions path — its native
	// endpoint is /inference (configurable server-side via
	// --inference-path), confirmed against a real local instance. Response
	// shape ({"text": "..."}) matches what whisperTranscriptionResponse
	// already expects.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.BaseURL+"/inference", body)
	if err != nil {
		return nil, fmt.Errorf("failed to build whisper.cpp request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := w.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call whisper.cpp transcription endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("whisper.cpp transcription endpoint returned status %d", resp.StatusCode)
	}

	var parsed whisperTranscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode whisper.cpp response: %w", err)
	}

	return &api.TranscriptionResponse{Text: parsed.Text, Model: whisperModelName}, nil
}
