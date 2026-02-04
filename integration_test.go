package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// binaryPath is set by TestMain after building the subline binary.
var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all integration tests.
	tmp, err := os.MkdirTemp("", "subline-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)

	binaryPath = filepath.Join(tmp, "subline")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build binary: " + err.Error())
	}

	os.Exit(m.Run())
}

func TestIntegrationSRT(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	cmd := exec.Command(binaryPath,
		"--output-dir", tmpDir,
		"--model", "tiny",
		"test/videos/fragment.mp4",
	)
	out, err := cmd.CombinedOutput()
	t.Log(string(out))
	if err != nil {
		t.Fatalf("subline exited with error: %v", err)
	}

	srtPath := filepath.Join(tmpDir, "fragment.srt")
	content, err := os.ReadFile(srtPath)
	if err != nil {
		t.Fatalf("SRT file not created: %v", err)
	}

	srt := string(content)
	// Should have at least one numbered entry.
	if !strings.Contains(srt, "1\n") {
		t.Error("SRT missing entry number")
	}
	// Should have timestamp arrows.
	if !strings.Contains(srt, " --> ") {
		t.Error("SRT missing timestamp separator")
	}
	// SRT uses commas in timestamps.
	if !strings.Contains(srt, ",") {
		t.Error("SRT timestamps should use commas")
	}
}

func TestIntegrationVTT(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	cmd := exec.Command(binaryPath,
		"--output-dir", tmpDir,
		"--format", "vtt",
		"--model", "tiny",
		"test/videos/fragment.mp4",
	)
	out, err := cmd.CombinedOutput()
	t.Log(string(out))
	if err != nil {
		t.Fatalf("subline exited with error: %v", err)
	}

	vttPath := filepath.Join(tmpDir, "fragment.vtt")
	content, err := os.ReadFile(vttPath)
	if err != nil {
		t.Fatalf("VTT file not created: %v", err)
	}

	vtt := string(content)
	if !strings.HasPrefix(vtt, "WEBVTT") {
		t.Error("VTT missing WEBVTT header")
	}
	if !strings.Contains(vtt, " --> ") {
		t.Error("VTT missing timestamp separator")
	}
}

func TestIntegrationSkipExisting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a dummy SRT file.
	srtPath := filepath.Join(tmpDir, "fragment.srt")
	os.WriteFile(srtPath, []byte("existing"), 0644)

	cmd := exec.Command(binaryPath,
		"--output-dir", tmpDir,
		"--skip-existing",
		"--model", "tiny",
		"test/videos/fragment.mp4",
	)
	out, err := cmd.CombinedOutput()
	t.Log(string(out))
	if err != nil {
		t.Fatalf("subline exited with error: %v", err)
	}

	// File should be unchanged.
	content, _ := os.ReadFile(srtPath)
	if string(content) != "existing" {
		t.Error("--skip-existing should have preserved the existing file")
	}
}

func TestIntegrationOutputDir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "subs")

	cmd := exec.Command(binaryPath,
		"--output-dir", outDir,
		"--model", "tiny",
		"test/videos/fragment.mp4",
	)
	out, err := cmd.CombinedOutput()
	t.Log(string(out))
	if err != nil {
		t.Fatalf("subline exited with error: %v", err)
	}

	srtPath := filepath.Join(outDir, "fragment.srt")
	if _, err := os.Stat(srtPath); err != nil {
		t.Fatalf("SRT not created in output dir: %v", err)
	}
}

func TestIntegrationNonexistentFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Should exit with non-zero since no valid files found.
	cmd := exec.Command(binaryPath, "/nonexistent/file.mp4")
	out, err := cmd.CombinedOutput()
	t.Log(string(out))
	if err == nil {
		t.Fatal("expected non-zero exit for no valid files")
	}
}
