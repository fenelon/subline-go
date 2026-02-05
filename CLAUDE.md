# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Subline

Subline is a CLI tool that generates subtitles (SRT/VTT) from video and audio files using OpenAI's Whisper speech recognition via whisper.cpp. It supports 99 languages with auto-detection, multi-track audio, batch processing, and Metal GPU acceleration on macOS. The output is a single statically-linked binary with zero runtime dependencies.

## Build Commands

```bash
make              # Full build: FFmpeg → whisper.cpp → Go binary (first build is slow)
make subline      # Rebuild only the Go binary (use after Go code changes)
make ffmpeg       # Build only FFmpeg static libraries
make whisper      # Build only whisper.cpp static libraries
make clean        # Remove all build artifacts
```

The binary is output to `./subline` in the project root.

**Prerequisites:** Go 1.23+, CMake, C/C++ compiler. Clone with `--recursive` for submodules.

## Testing

```bash
go test ./...              # All tests (integration tests require built binary + test videos)
go test -short ./...       # Unit tests only (skip integration)
go test -run TestXxx ./... # Run a specific test
go test -v ./...           # Verbose output
```

Integration tests (`integration_test.go`) use `TestMain` to build the binary, then run it against files in `test/videos/`. They are skipped with `-short`.

**Note:** Running `go test` requires the C libraries (FFmpeg, whisper.cpp) to already be built. Run `make ffmpeg whisper` first if you haven't done a full build.

## Architecture

All Go source files are in a single `main` package at the root. There are no subdirectories or internal packages.

### Pipeline Flow

```
main.go           CLI entry, flag parsing, orchestrates the full pipeline
  → discover.go   FindMediaFiles() - recursively finds supported media files
  → models.go     EnsureModel() - downloads Whisper GGML models to cache dir, with retry
  → whisper.go    LoadModel() - loads model via CGo into whisper.cpp context
  → audio.go      ProbeAudioTracks() / ExtractAudio() - FFmpeg via go-astiav, resamples to 16kHz mono f32
  → track.go      PickAudioTracks() - interactive track selection for multi-track files
  → transcribe.go DetectLanguage() - uses first 30s of audio for language detection
  → whisper.go    Transcribe() - runs inference, returns []Segment with timestamps
  → subtitle.go   WriteSRT() / WriteVTT() - formats and writes subtitle files
  → progress.go   Progress bar and SIGINT cleanup of partial files
```

### CGo Bindings

`whisper.go` contains the CGo bridge to whisper.cpp. It uses a trampoline pattern for progress callbacks (C can't call Go functions directly, so an ID-based registry maps C callbacks to Go functions). `WhisperModel` is **not** safe for concurrent use.

`audio.go` uses `go-astiav` (a Go wrapper around FFmpeg's libav* libraries) for audio probing and extraction. All C libraries are linked statically.

### C Output Suppression

`main.go` contains `suppressCOutput()` which redirects C-level stdout/stderr to `/dev/null` (unless `--verbose`). A duplicate stderr fd (`realStderr`) is kept for the progress bar. This is why progress output works even when C libraries are silenced.

### Build System

The `Makefile` handles platform detection (Darwin vs Linux):
- **macOS:** Enables Metal GPU (GGML_METAL=ON), links Accelerate/Metal/Foundation frameworks
- **Linux:** CPU-only (GGML_METAL=OFF)

FFmpeg is configured with minimal demuxers/decoders for audio-only use. Both FFmpeg and whisper.cpp are vendored as git submodules in `third_party/` and built as static archives.

### Testing Patterns

- Unit tests use overrideable global variables for dependency injection (`cacheDirOverride`, `modelBaseURLOverride` in models.go)
- Integration tests build the full binary in `TestMain` and invoke it as a subprocess
- Tests use `testing.Short()` to skip slow/integration tests
