package main

import "testing"

func TestPickAudioTrackManual(t *testing.T) {
	tracks := []AudioTrack{
		{StreamIndex: 1, Language: "eng"},
		{StreamIndex: 2, Language: "spa"},
	}
	idx, err := PickAudioTrack(tracks, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 2 {
		t.Errorf("expected 2, got %d", idx)
	}
}

func TestPickAudioTrackSingle(t *testing.T) {
	tracks := []AudioTrack{
		{StreamIndex: 0, Language: "eng"},
	}
	idx, err := PickAudioTrack(tracks, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 0 {
		t.Errorf("expected 0, got %d", idx)
	}
}

func TestPickAudioTrackMultiple(t *testing.T) {
	tracks := []AudioTrack{
		{StreamIndex: 1, Language: "eng"},
		{StreamIndex: 2, Language: "spa"},
	}
	idx, err := PickAudioTrack(tracks, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return first track.
	if idx != 1 {
		t.Errorf("expected 1, got %d", idx)
	}
}

func TestPickAudioTrackNoTracks(t *testing.T) {
	_, err := PickAudioTrack(nil, -1)
	if err == nil {
		t.Fatal("expected error for no tracks")
	}
}

func TestPickAudioTrackUndLanguage(t *testing.T) {
	tracks := []AudioTrack{
		{StreamIndex: 0, Language: ""},
	}
	idx, err := PickAudioTrack(tracks, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 0 {
		t.Errorf("expected 0, got %d", idx)
	}
}
