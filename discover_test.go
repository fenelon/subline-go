package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// helper: create a temp file inside dir with the given name (empty content).
func touchFile(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, nil, 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFindMediaFiles_SingleFile(t *testing.T) {
	dir := t.TempDir()
	mp4 := touchFile(t, dir, "video.mp4")

	got := FindMediaFiles([]string{mp4})
	if len(got) != 1 {
		t.Fatalf("expected 1 file, got %d", len(got))
	}
	if got[0] != mp4 {
		t.Errorf("expected %q, got %q", mp4, got[0])
	}
}

func TestFindMediaFiles_Directory(t *testing.T) {
	dir := t.TempDir()
	aMP4 := touchFile(t, dir, "a.mp4")
	bMKV := touchFile(t, dir, "b.mkv")
	touchFile(t, dir, "c.txt") // not a media file
	dAVI := touchFile(t, dir, "d.avi")

	got := FindMediaFiles([]string{dir})
	want := []string{aMP4, bMKV, dAVI}
	if len(got) != len(want) {
		t.Fatalf("expected %d files, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestFindMediaFiles_MultiplePaths(t *testing.T) {
	dir := t.TempDir()
	f1 := touchFile(t, dir, "one.mp4")
	f2 := touchFile(t, dir, "two.mkv")

	got := FindMediaFiles([]string{f1, f2})
	if len(got) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(got), got)
	}
	if got[0] != f1 || got[1] != f2 {
		t.Errorf("expected [%q, %q], got %v", f1, f2, got)
	}
}

func TestFindMediaFiles_NonexistentPath(t *testing.T) {
	// Capture stderr to verify warning is printed.
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	got := FindMediaFiles([]string{"/no/such/path/fake.mp4"})

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stderr = old

	if len(got) != 0 {
		t.Fatalf("expected 0 files, got %d: %v", len(got), got)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Warning")) {
		t.Errorf("expected warning on stderr, got %q", buf.String())
	}
}

func TestFindMediaFiles_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	got := FindMediaFiles([]string{dir})
	if len(got) != 0 {
		t.Fatalf("expected 0 files, got %d: %v", len(got), got)
	}
}

func TestFindMediaFiles_AudioFile(t *testing.T) {
	dir := t.TempDir()
	wav := touchFile(t, dir, "audio.wav")

	got := FindMediaFiles([]string{wav})
	if len(got) != 1 {
		t.Fatalf("expected 1 file, got %d", len(got))
	}
	if got[0] != wav {
		t.Errorf("expected %q, got %q", wav, got[0])
	}
}
