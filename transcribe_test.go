package main

import (
	"math"
	"testing"
)

// generateSineWave produces a mono 16 kHz sine wave at the given frequency
// for durationSec seconds.  The result is suitable for passing directly to
// WhisperModel.Transcribe.
func generateSineWave(freq float64, sampleRate, durationSec int) []float32 {
	n := sampleRate * durationSec
	samples := make([]float32, n)
	for i := range samples {
		samples[i] = float32(math.Sin(2 * math.Pi * freq * float64(i) / float64(sampleRate)))
	}
	return samples
}

// generateSilence produces a slice of zero-valued samples (silence).
func generateSilence(sampleRate, durationSec int) []float32 {
	return make([]float32, sampleRate*durationSec)
}

// TestTranscribe loads the tiny whisper model and runs transcription on a
// synthetic sine wave.  The test verifies the API contract (model loads,
// transcription returns without error, segment timestamps are sane) rather
// than the linguistic quality of the output.
func TestTranscribe(t *testing.T) {
	modelPath, err := EnsureModel("tiny")
	if err != nil {
		t.Skip("Could not obtain tiny model:", err)
	}

	model, err := LoadModel(modelPath)
	if err != nil {
		t.Fatal("Failed to load model:", err)
	}
	defer model.Close()

	// 3 seconds of 440 Hz sine wave at 16 kHz (whisper's expected sample rate).
	samples := generateSineWave(440, 16000, 3)

	segments, err := model.Transcribe(samples, "", nil)
	if err != nil {
		t.Fatal("Transcription failed:", err)
	}

	// Whisper should produce at least one segment for any non-trivial input.
	if len(segments) == 0 {
		t.Fatal("Expected at least one segment")
	}

	for i, seg := range segments {
		if seg.End < seg.Start {
			t.Errorf("segment %d: end (%v) before start (%v)", i, seg.End, seg.Start)
		}
	}
}

// TestTranscribeSilence feeds pure silence to verify the API handles quiet
// input gracefully (no crash, no error).
func TestTranscribeSilence(t *testing.T) {
	modelPath, err := EnsureModel("tiny")
	if err != nil {
		t.Skip("Could not obtain tiny model:", err)
	}

	model, err := LoadModel(modelPath)
	if err != nil {
		t.Fatal("Failed to load model:", err)
	}
	defer model.Close()

	samples := generateSilence(16000, 2)

	segments, err := model.Transcribe(samples, "", nil)
	if err != nil {
		t.Fatal("Transcription on silence failed:", err)
	}

	// Segments may or may not be produced for silence; the key assertion is
	// that we did not crash.
	for i, seg := range segments {
		if seg.End < seg.Start {
			t.Errorf("segment %d: end (%v) before start (%v)", i, seg.End, seg.Start)
		}
	}
}

// TestDetectLanguage ensures the standalone language detection works
// without running full transcription.
func TestDetectLanguage(t *testing.T) {
	modelPath, err := EnsureModel("tiny")
	if err != nil {
		t.Skip("Could not obtain tiny model:", err)
	}

	model, err := LoadModel(modelPath)
	if err != nil {
		t.Fatal("Failed to load model:", err)
	}
	defer model.Close()

	samples := generateSineWave(440, 16000, 3)

	lang := model.DetectLanguage(samples)
	if lang == "" {
		t.Error("DetectLanguage returned empty string")
	}
	t.Logf("Detected language: %s", lang)
}

// TestTranscribeWithLanguage exercises the explicit language parameter.
func TestTranscribeWithLanguage(t *testing.T) {
	modelPath, err := EnsureModel("tiny")
	if err != nil {
		t.Skip("Could not obtain tiny model:", err)
	}

	model, err := LoadModel(modelPath)
	if err != nil {
		t.Fatal("Failed to load model:", err)
	}
	defer model.Close()

	samples := generateSineWave(440, 16000, 3)

	segments, err := model.Transcribe(samples, "en", nil)
	if err != nil {
		t.Fatal("Transcription with language=en failed:", err)
	}

	if len(segments) == 0 {
		t.Fatal("Expected at least one segment with explicit language")
	}
}

// TestLoadModelInvalid verifies that LoadModel returns an error for a
// non-existent file rather than crashing.
func TestLoadModelInvalid(t *testing.T) {
	_, err := LoadModel("/nonexistent/model.bin")
	if err == nil {
		t.Fatal("Expected error for non-existent model path")
	}
}

// TestTranscribeEmptySamples verifies that Transcribe returns an error
// when given an empty sample slice.
func TestTranscribeEmptySamples(t *testing.T) {
	modelPath, err := EnsureModel("tiny")
	if err != nil {
		t.Skip("Could not obtain tiny model:", err)
	}

	model, err := LoadModel(modelPath)
	if err != nil {
		t.Fatal("Failed to load model:", err)
	}
	defer model.Close()

	_, err = model.Transcribe([]float32{}, "", nil)
	if err == nil {
		t.Fatal("Expected error for empty samples")
	}
}

// TestIsMultilingual checks the model capability query.
func TestIsMultilingual(t *testing.T) {
	modelPath, err := EnsureModel("tiny")
	if err != nil {
		t.Skip("Could not obtain tiny model:", err)
	}

	model, err := LoadModel(modelPath)
	if err != nil {
		t.Fatal("Failed to load model:", err)
	}
	defer model.Close()

	// The "tiny" model (ggml-tiny.bin) is multilingual.
	if !model.IsMultilingual() {
		t.Error("Expected tiny model to be multilingual")
	}
}

// TestModelDoubleClose verifies that calling Close twice does not panic.
func TestModelDoubleClose(t *testing.T) {
	modelPath, err := EnsureModel("tiny")
	if err != nil {
		t.Skip("Could not obtain tiny model:", err)
	}

	model, err := LoadModel(modelPath)
	if err != nil {
		t.Fatal("Failed to load model:", err)
	}

	model.Close()
	model.Close() // must not panic
}

// TestTranscribeAfterClose verifies that Transcribe on a closed model
// returns an error rather than crashing.
func TestTranscribeAfterClose(t *testing.T) {
	modelPath, err := EnsureModel("tiny")
	if err != nil {
		t.Skip("Could not obtain tiny model:", err)
	}

	model, err := LoadModel(modelPath)
	if err != nil {
		t.Fatal("Failed to load model:", err)
	}
	model.Close()

	samples := generateSineWave(440, 16000, 1)
	_, err = model.Transcribe(samples, "", nil)
	if err == nil {
		t.Fatal("Expected error when transcribing after Close")
	}
}
