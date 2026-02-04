package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// CacheDir tests
// ---------------------------------------------------------------------------

func TestModelCacheDir_Suffix(t *testing.T) {
	dir := CacheDir()
	wantSuffix := filepath.Join("subline", "models")
	if !strings.HasSuffix(dir, wantSuffix) {
		t.Errorf("CacheDir() = %q; want suffix %q", dir, wantSuffix)
	}
}

func TestModelCacheDir_Platform(t *testing.T) {
	dir := CacheDir()
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(dir, filepath.Join("Library", "Caches")) {
			t.Errorf("on macOS, CacheDir() = %q; want path containing Library/Caches", dir)
		}
	default:
		// Linux / other: should use XDG_CACHE_HOME or ~/.cache
		xdg := os.Getenv("XDG_CACHE_HOME")
		if xdg != "" {
			if !strings.HasPrefix(dir, xdg) {
				t.Errorf("CacheDir() = %q; want prefix %q (XDG_CACHE_HOME)", dir, xdg)
			}
		} else {
			home, _ := os.UserHomeDir()
			wantPrefix := filepath.Join(home, ".cache")
			if !strings.HasPrefix(dir, wantPrefix) {
				t.Errorf("CacheDir() = %q; want prefix %q", dir, wantPrefix)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// ModelFileName tests
// ---------------------------------------------------------------------------

func TestModelFileName_Turbo(t *testing.T) {
	got, err := ModelFileName("turbo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ggml-large-v3-turbo.bin" {
		t.Errorf("ModelFileName(turbo) = %q; want %q", got, "ggml-large-v3-turbo.bin")
	}
}

func TestModelFileName_Large(t *testing.T) {
	got, err := ModelFileName("large")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ggml-large-v3.bin" {
		t.Errorf("ModelFileName(large) = %q; want %q", got, "ggml-large-v3.bin")
	}
}

func TestModelFileName_Small(t *testing.T) {
	got, err := ModelFileName("small")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ggml-small.bin" {
		t.Errorf("ModelFileName(small) = %q; want %q", got, "ggml-small.bin")
	}
}

func TestModelFileName_Tiny(t *testing.T) {
	got, err := ModelFileName("tiny")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ggml-tiny.bin" {
		t.Errorf("ModelFileName(tiny) = %q; want %q", got, "ggml-tiny.bin")
	}
}

func TestModelFileName_Base(t *testing.T) {
	got, err := ModelFileName("base")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ggml-base.bin" {
		t.Errorf("ModelFileName(base) = %q; want %q", got, "ggml-base.bin")
	}
}

func TestModelFileName_Medium(t *testing.T) {
	got, err := ModelFileName("medium")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ggml-medium.bin" {
		t.Errorf("ModelFileName(medium) = %q; want %q", got, "ggml-medium.bin")
	}
}

func TestModelFileName_Invalid(t *testing.T) {
	_, err := ModelFileName("invalid")
	if err == nil {
		t.Error("ModelFileName(invalid) should return an error")
	}
}

// ---------------------------------------------------------------------------
// ModelURL tests
// ---------------------------------------------------------------------------

func TestModelURL_Turbo(t *testing.T) {
	got, err := ModelURL("turbo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin"
	if got != want {
		t.Errorf("ModelURL(turbo) = %q; want %q", got, want)
	}
}

func TestModelURL_Invalid(t *testing.T) {
	_, err := ModelURL("invalid")
	if err == nil {
		t.Error("ModelURL(invalid) should return an error")
	}
}

// ---------------------------------------------------------------------------
// EnsureModel tests
// ---------------------------------------------------------------------------

func TestModelEnsure_ExistingFile(t *testing.T) {
	// Create a temp directory to act as cache
	tmpDir := t.TempDir()
	origCacheDir := cacheDirOverride
	cacheDirOverride = tmpDir
	t.Cleanup(func() { cacheDirOverride = origCacheDir })

	// Pre-create the model file
	fname, _ := ModelFileName("tiny")
	fpath := filepath.Join(tmpDir, fname)
	if err := os.WriteFile(fpath, []byte("fake-model-data"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := EnsureModel("tiny")
	if err != nil {
		t.Fatalf("EnsureModel returned error: %v", err)
	}
	if got != fpath {
		t.Errorf("EnsureModel(tiny) = %q; want %q", got, fpath)
	}
}

func TestModelEnsure_DownloadsMissing(t *testing.T) {
	// Set up a mock HTTP server that serves fake model data.
	modelData := []byte("fake-ggml-model-binary-content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(modelData)))
		w.WriteHeader(http.StatusOK)
		w.Write(modelData)
	}))
	defer srv.Close()

	// Use a temp directory as cache and override the base URL.
	tmpDir := t.TempDir()
	origCacheDir := cacheDirOverride
	origBaseURL := modelBaseURLOverride
	cacheDirOverride = tmpDir
	modelBaseURLOverride = srv.URL + "/"
	t.Cleanup(func() {
		cacheDirOverride = origCacheDir
		modelBaseURLOverride = origBaseURL
	})

	got, err := EnsureModel("tiny")
	if err != nil {
		t.Fatalf("EnsureModel returned error: %v", err)
	}

	wantPath := filepath.Join(tmpDir, "ggml-tiny.bin")
	if got != wantPath {
		t.Errorf("EnsureModel(tiny) = %q; want %q", got, wantPath)
	}

	// Verify the file was actually written.
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(data) != string(modelData) {
		t.Errorf("downloaded file content = %q; want %q", string(data), string(modelData))
	}
}

func TestModelEnsure_InvalidModel(t *testing.T) {
	_, err := EnsureModel("nonexistent")
	if err == nil {
		t.Error("EnsureModel(nonexistent) should return an error")
	}
}

func TestModelEnsure_HTTPError(t *testing.T) {
	// Set up a mock HTTP server that returns 404.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	origCacheDir := cacheDirOverride
	origBaseURL := modelBaseURLOverride
	cacheDirOverride = tmpDir
	modelBaseURLOverride = srv.URL + "/"
	t.Cleanup(func() {
		cacheDirOverride = origCacheDir
		modelBaseURLOverride = origBaseURL
	})

	_, err := EnsureModel("tiny")
	if err == nil {
		t.Error("EnsureModel should return error on HTTP 404")
	}
}
