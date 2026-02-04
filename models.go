package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

// modelFiles maps friendly model names to their GGML filenames.
var modelFiles = map[string]string{
	"tiny":   "ggml-tiny.bin",
	"base":   "ggml-base.bin",
	"small":  "ggml-small.bin",
	"medium": "ggml-medium.bin",
	"turbo":  "ggml-large-v3-turbo.bin",
	"large":  "ggml-large-v3.bin",
}

const defaultModelBaseURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/"

// cacheDirOverride allows tests to redirect the cache directory.
var cacheDirOverride string

// modelBaseURLOverride allows tests to redirect model downloads.
var modelBaseURLOverride string

// CacheDir returns the platform-appropriate cache directory for model files.
//   - macOS: ~/Library/Caches/subline/models
//   - Linux: $XDG_CACHE_HOME/subline/models (or ~/.cache/subline/models)
func CacheDir() string {
	if cacheDirOverride != "" {
		return cacheDirOverride
	}

	var base string
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "Library", "Caches")
	default:
		if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
			base = xdg
		} else {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, ".cache")
		}
	}
	return filepath.Join(base, "subline", "models")
}

// ModelFileName returns the GGML filename for the given model name.
func ModelFileName(name string) (string, error) {
	f, ok := modelFiles[name]
	if !ok {
		return "", fmt.Errorf("unknown model %q; valid models: tiny, base, small, medium, turbo, large", name)
	}
	return f, nil
}

// ModelURL returns the HuggingFace download URL for the given model name.
func ModelURL(name string) (string, error) {
	f, err := ModelFileName(name)
	if err != nil {
		return "", err
	}
	base := defaultModelBaseURL
	if modelBaseURLOverride != "" {
		base = modelBaseURLOverride
	}
	return base + f, nil
}

// EnsureModel checks whether the model file already exists in the cache.
// If it does, it returns the path immediately. Otherwise it downloads
// the model from HuggingFace with progress output to stderr and returns
// the local path. It retries once on download failure.
func EnsureModel(name string) (string, error) {
	fname, err := ModelFileName(name)
	if err != nil {
		return "", err
	}

	dir := CacheDir()
	fpath := filepath.Join(dir, fname)

	// If the file already exists, return immediately.
	if _, err := os.Stat(fpath); err == nil {
		return fpath, nil
	}

	// Ensure the cache directory exists.
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating cache directory: %w", err)
	}

	url, err := ModelURL(name)
	if err != nil {
		return "", err
	}

	// Try download, retry once on failure.
	var dlErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			fmt.Fprintf(os.Stderr, "Retrying download (attempt %d)...\n", attempt+1)
		}
		dlErr = downloadModel(url, fpath, name)
		if dlErr == nil {
			return fpath, nil
		}
	}

	return "", fmt.Errorf("downloading model %q: %w", name, dlErr)
}

// downloadModel fetches the model from url and writes it to dest,
// printing progress to stderr.
func downloadModel(url, dest, name string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}

	totalBytes := resp.ContentLength
	if totalBytes > 0 {
		sizeMB := totalBytes / (1024 * 1024)
		fmt.Fprintf(os.Stderr, "Downloading model '%s' (%d MB)...\n", name, sizeMB)
	} else {
		// Try Content-Length header as fallback.
		if cl := resp.Header.Get("Content-Length"); cl != "" {
			if n, parseErr := strconv.ParseInt(cl, 10, 64); parseErr == nil {
				totalBytes = n
				sizeMB := totalBytes / (1024 * 1024)
				fmt.Fprintf(os.Stderr, "Downloading model '%s' (%d MB)...\n", name, sizeMB)
			}
		}
		if totalBytes <= 0 {
			fmt.Fprintf(os.Stderr, "Downloading model '%s'...\n", name)
		}
	}

	// Write to a temporary file first, then rename (atomic-ish).
	tmpPath := dest + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer func() {
		out.Close()
		os.Remove(tmpPath) // clean up on failure; no-op if already renamed
	}()

	// Wrap the reader with a progress counter.
	reader := &progressReader{
		reader:     resp.Body,
		totalBytes: totalBytes,
		name:       name,
	}

	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("writing model data: %w", err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Rename temp file to final destination.
	if err := os.Rename(tmpPath, dest); err != nil {
		return fmt.Errorf("moving temp file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Model '%s' downloaded successfully.\n", name)
	return nil
}

// progressReader wraps an io.Reader and prints download progress to stderr.
type progressReader struct {
	reader      io.Reader
	totalBytes  int64
	read        int64
	name        string
	lastPercent int
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.read += int64(n)

	if pr.totalBytes > 0 {
		percent := int(pr.read * 100 / pr.totalBytes)
		// Print at every 10% increment to avoid excessive output.
		if percent/10 > pr.lastPercent/10 {
			fmt.Fprintf(os.Stderr, "  %d%%\n", percent)
			pr.lastPercent = percent
		}
	}

	return n, err
}
