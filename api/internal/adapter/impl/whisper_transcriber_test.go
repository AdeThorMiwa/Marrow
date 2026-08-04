package adapter

import (
	"context"
	"os"
	"testing"

	model "marrow/internal/model"
)

// TestWhisperCppTranscriber_RealServer_EndToEnd hits a real local
// whisper.cpp server (per this repo's convention of testing against real
// infra rather than mocking) — requires `whisper-server` running on
// localhost:8081 with the `medium` model and --convert enabled. testdata/speech.wav
// says "Marrow is a personal content retention app that helps you remember
// what you read and watch."
func TestWhisperCppTranscriber_RealServer_EndToEnd(t *testing.T) {
	buf, err := os.ReadFile("testdata/speech.wav")
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	tr := NewWhisperCppTranscriber("http://localhost:8081")
	resp, err := tr.Transcribe(context.Background(), model.Media{Buffer: buf, Kind: model.MediaAudio})
	if err != nil {
		t.Fatalf("transcribe failed (is whisper-server running on :8081?): %v", err)
	}

	if resp.Text == "" {
		t.Fatal("expected non-empty transcription")
	}
	if resp.Model != "whisper-medium" {
		t.Errorf("expected model %q, got %q", "whisper-medium", resp.Model)
	}
	t.Logf("transcribed: %q", resp.Text)
}
