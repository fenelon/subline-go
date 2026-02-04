package main

import (
	"testing"
)

const testVideoPath = "test/videos/fragment.mp4"

func TestProbeAudioTracks(t *testing.T) {
	tracks, err := ProbeAudioTracks(testVideoPath)
	if err != nil {
		t.Fatalf("ProbeAudioTracks(%q) returned error: %v", testVideoPath, err)
	}

	if len(tracks) == 0 {
		t.Fatalf("expected at least one audio track, got 0")
	}

	tr := tracks[0]
	if tr.StreamIndex < 0 {
		t.Errorf("expected non-negative stream index, got %d", tr.StreamIndex)
	}
	if tr.Channels <= 0 {
		t.Errorf("expected positive channel count, got %d", tr.Channels)
	}
	if tr.SampleRate <= 0 {
		t.Errorf("expected positive sample rate, got %d", tr.SampleRate)
	}
	if tr.Codec == "" {
		t.Errorf("expected non-empty codec name")
	}

	t.Logf("found %d audio track(s):", len(tracks))
	for i, tr := range tracks {
		t.Logf("  [%d] stream=%d lang=%q codec=%s ch=%d rate=%d",
			i, tr.StreamIndex, tr.Language, tr.Codec, tr.Channels, tr.SampleRate)
	}
}

func TestProbeAudioTracks_NonExistent(t *testing.T) {
	_, err := ProbeAudioTracks("/nonexistent/file.mp4")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
	t.Logf("got expected error: %v", err)
}

func TestExtractAudio(t *testing.T) {
	tracks, err := ProbeAudioTracks(testVideoPath)
	if err != nil {
		t.Fatalf("ProbeAudioTracks: %v", err)
	}
	if len(tracks) == 0 {
		t.Fatal("no audio tracks found for extraction test")
	}

	samples, err := ExtractAudio(testVideoPath, tracks[0].StreamIndex)
	if err != nil {
		t.Fatalf("ExtractAudio returned error: %v", err)
	}

	if len(samples) == 0 {
		t.Fatal("expected non-empty float32 slice, got length 0")
	}

	t.Logf("extracted %d samples (%.2f seconds at 16kHz)", len(samples), float64(len(samples))/16000.0)
}

func TestExtractAudio_HasNonZeroSamples(t *testing.T) {
	tracks, err := ProbeAudioTracks(testVideoPath)
	if err != nil {
		t.Fatalf("ProbeAudioTracks: %v", err)
	}
	if len(tracks) == 0 {
		t.Fatal("no audio tracks found for extraction test")
	}

	samples, err := ExtractAudio(testVideoPath, tracks[0].StreamIndex)
	if err != nil {
		t.Fatalf("ExtractAudio returned error: %v", err)
	}

	nonZero := 0
	for _, s := range samples {
		if s != 0 {
			nonZero++
		}
	}

	if nonZero == 0 {
		t.Fatal("all samples are zero; expected at least some non-zero samples")
	}

	pct := float64(nonZero) / float64(len(samples)) * 100
	t.Logf("%d/%d samples are non-zero (%.1f%%)", nonZero, len(samples), pct)
}
