package main

import (
	"os"
	"strings"
	"testing"
)

func TestPickAudioTracksManual(t *testing.T) {
	tracks := []AudioTrack{
		{StreamIndex: 1, Language: "eng"},
		{StreamIndex: 2, Language: "spa"},
	}
	indices, err := PickAudioTracks(tracks, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(indices) != 1 || indices[0] != 2 {
		t.Errorf("expected [2], got %v", indices)
	}
}

func TestPickAudioTracksSingle(t *testing.T) {
	tracks := []AudioTrack{
		{StreamIndex: 0, Language: "eng"},
	}
	indices, err := PickAudioTracks(tracks, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(indices) != 1 || indices[0] != 0 {
		t.Errorf("expected [0], got %v", indices)
	}
}

func TestPickAudioTracksInteractive(t *testing.T) {
	tracks := []AudioTrack{
		{StreamIndex: 1, Language: "eng", Codec: "aac", Channels: 2, SampleRate: 48000},
		{StreamIndex: 2, Language: "spa", Codec: "aac", Channels: 2, SampleRate: 48000},
	}

	// Simulate user typing "2\n" on stdin.
	r, w, _ := os.Pipe()
	w.WriteString("2\n")
	w.Close()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	indices, err := PickAudioTracks(tracks, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(indices) != 1 || indices[0] != 2 {
		t.Errorf("expected [2], got %v", indices)
	}
}

func TestPickAudioTracksAll(t *testing.T) {
	tracks := []AudioTrack{
		{StreamIndex: 1, Language: "eng", Codec: "aac", Channels: 2, SampleRate: 48000},
		{StreamIndex: 2, Language: "spa", Codec: "aac", Channels: 2, SampleRate: 48000},
	}

	// Simulate user typing "a\n".
	r, w, _ := os.Pipe()
	w.WriteString("a\n")
	w.Close()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	indices, err := PickAudioTracks(tracks, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(indices) != 2 || indices[0] != 1 || indices[1] != 2 {
		t.Errorf("expected [1 2], got %v", indices)
	}
}

func TestPickAudioTracksInvalidThenValid(t *testing.T) {
	tracks := []AudioTrack{
		{StreamIndex: 1, Language: "eng", Codec: "aac", Channels: 2, SampleRate: 48000},
		{StreamIndex: 2, Language: "spa", Codec: "aac", Channels: 2, SampleRate: 48000},
	}

	// Simulate user typing bad input, then valid choice.
	r, w, _ := os.Pipe()
	w.WriteString("bad\n5\n1\n")
	w.Close()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	// Capture stdout to verify error messages were printed.
	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut
	defer func() { os.Stdout = oldStdout }()

	indices, err := PickAudioTracks(tracks, -1)
	wOut.Close()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(indices) != 1 || indices[0] != 1 {
		t.Errorf("expected [1], got %v", indices)
	}

	buf := make([]byte, 4096)
	n, _ := rOut.Read(buf)
	output := string(buf[:n])
	if !strings.Contains(output, "Invalid choice") {
		t.Error("expected 'Invalid choice' message for bad input")
	}
}

func TestPickAudioTracksNoTracks(t *testing.T) {
	_, err := PickAudioTracks(nil, -1)
	if err == nil {
		t.Fatal("expected error for no tracks")
	}
}

func TestPickAudioTracksUndLanguage(t *testing.T) {
	tracks := []AudioTrack{
		{StreamIndex: 0, Language: ""},
	}
	indices, err := PickAudioTracks(tracks, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(indices) != 1 || indices[0] != 0 {
		t.Errorf("expected [0], got %v", indices)
	}
}

func TestTrackLanguage(t *testing.T) {
	tracks := []AudioTrack{
		{StreamIndex: 1, Language: "eng"},
		{StreamIndex: 2, Language: ""},
	}
	if got := TrackLanguage(tracks, 1); got != "eng" {
		t.Errorf("expected eng, got %s", got)
	}
	if got := TrackLanguage(tracks, 2); got != "und" {
		t.Errorf("expected und, got %s", got)
	}
	if got := TrackLanguage(tracks, 99); got != "und" {
		t.Errorf("expected und for missing, got %s", got)
	}
}
