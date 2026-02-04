# subline-go Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rewrite the subline subtitle generator in Go as a single static binary with whisper.cpp (Metal on macOS) and embedded FFmpeg.

**Architecture:** Makefile builds FFmpeg (static) and whisper.cpp (static, Metal on macOS) from source, then `go build` links everything into one binary. CLI uses stdlib `flag`. Audio decoded in-process via astiav. Models auto-downloaded from Hugging Face on first use.

**Tech Stack:** Go 1.22+, whisper.cpp (git submodule), FFmpeg n8.0 (git submodule), astiav, CGo

**Reference:** Python source at `/Users/ellin/code/subline/subline.py`, design doc at `docs/plans/2026-02-04-subline-go-design.md`

---

### Task 1: Project scaffolding — git init, go mod, submodules

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `third_party/` (submodules)

**Step 1: Initialize git repo**

```bash
cd /Users/ellin/code/subline-go
git init
```

**Step 2: Create .gitignore**

```
# Build artifacts
/subline
/build/

# Models
models/

# OS
.DS_Store

# IDE
.idea/
.vscode/
```

**Step 3: Initialize Go module**

```bash
go mod init github.com/ellin/subline
```

**Step 4: Add whisper.cpp as git submodule**

```bash
git submodule add https://github.com/ggml-org/whisper.cpp.git third_party/whisper.cpp
cd third_party/whisper.cpp && git checkout v1.8.3 && cd ../..
```

**Step 5: Add FFmpeg as git submodule**

```bash
git submodule add https://github.com/FFmpeg/FFmpeg.git third_party/FFmpeg
cd third_party/FFmpeg && git checkout n7.1 && cd ../..
```

Note: Use FFmpeg n7.1 (latest stable release compatible with astiav). Check astiav docs for exact version compatibility — astiav v0.22.0 targets FFmpeg n7.1.

**Step 6: Initial commit**

```bash
git add .
git commit -m "chore: project scaffolding with whisper.cpp and FFmpeg submodules"
```

---

### Task 2: Makefile — build FFmpeg static libs, whisper.cpp static lib, then Go binary

**Files:**
- Create: `Makefile`

**Step 1: Write the Makefile**

The Makefile needs to:
1. Build FFmpeg as static libraries (libavcodec.a, libavformat.a, libswresample.a, libavutil.a)
2. Build whisper.cpp as static library (libwhisper.a + ggml libs)
3. Set CGo environment variables and build the Go binary

```makefile
UNAME_S := $(shell uname -s)
ROOT_DIR := $(shell pwd)

# FFmpeg
FFMPEG_SRC := $(ROOT_DIR)/third_party/FFmpeg
FFMPEG_BUILD := $(ROOT_DIR)/build/ffmpeg
FFMPEG_LIBS := $(FFMPEG_BUILD)/lib

# whisper.cpp
WHISPER_SRC := $(ROOT_DIR)/third_party/whisper.cpp
WHISPER_BUILD := $(ROOT_DIR)/build/whisper

# Platform-specific flags
ifeq ($(UNAME_S),Darwin)
    WHISPER_CMAKE_FLAGS := -DGGML_METAL=ON -DGGML_METAL_EMBED_LIBRARY=ON
    FRAMEWORKS := -framework Accelerate -framework Metal -framework Foundation -framework CoreGraphics
    FFMPEG_EXTRA_FLAGS := --enable-videotoolbox --enable-audiotoolbox
else
    WHISPER_CMAKE_FLAGS := -DGGML_METAL=OFF
    FRAMEWORKS :=
    FFMPEG_EXTRA_FLAGS :=
endif

.PHONY: all clean ffmpeg whisper subline

all: subline

# Build FFmpeg static libraries
ffmpeg: $(FFMPEG_LIBS)/libavcodec.a

$(FFMPEG_LIBS)/libavcodec.a:
	@echo "==> Building FFmpeg static libraries..."
	mkdir -p $(FFMPEG_BUILD)
	cd $(FFMPEG_SRC) && ./configure \
		--prefix=$(FFMPEG_BUILD) \
		--disable-shared \
		--enable-static \
		--disable-programs \
		--disable-doc \
		--disable-network \
		--disable-everything \
		--enable-demuxer=mov,matroska,avi,wav,flac,mp3 \
		--enable-decoder=aac,mp3,flac,opus,vorbis,pcm_s16le,pcm_f32le,ac3,eac3,dts \
		--enable-parser=aac,mpegaudio,flac,opus,vorbis,ac3,dts \
		--enable-protocol=file \
		--enable-filter=aresample \
		--enable-swresample \
		$(FFMPEG_EXTRA_FLAGS) \
		--disable-autodetect
	$(MAKE) -C $(FFMPEG_SRC) -j$(shell nproc 2>/dev/null || sysctl -n hw.logicalcpu)
	$(MAKE) -C $(FFMPEG_SRC) install

# Build whisper.cpp static library
whisper: $(WHISPER_BUILD)/lib/libwhisper.a

$(WHISPER_BUILD)/lib/libwhisper.a:
	@echo "==> Building whisper.cpp..."
	cmake -B $(WHISPER_BUILD) -S $(WHISPER_SRC) \
		-DCMAKE_BUILD_TYPE=Release \
		-DCMAKE_INSTALL_PREFIX=$(WHISPER_BUILD) \
		-DBUILD_SHARED_LIBS=OFF \
		-DWHISPER_BUILD_TESTS=OFF \
		-DWHISPER_BUILD_EXAMPLES=OFF \
		-DWHISPER_BUILD_SERVER=OFF \
		$(WHISPER_CMAKE_FLAGS)
	cmake --build $(WHISPER_BUILD) --config Release -j$(shell nproc 2>/dev/null || sysctl -n hw.logicalcpu)
	cmake --install $(WHISPER_BUILD) --config Release

# Build Go binary
subline: ffmpeg whisper
	@echo "==> Building subline..."
	CGO_ENABLED=1 \
	CGO_CFLAGS="-I$(WHISPER_BUILD)/include -I$(FFMPEG_BUILD)/include" \
	CGO_LDFLAGS="-L$(WHISPER_BUILD)/lib -L$(FFMPEG_LIBS) -lwhisper -lggml -lggml-base -lggml-cpu -lm -lstdc++ $(FRAMEWORKS) -lavcodec -lavformat -lswresample -lavutil -lswscale -lpthread" \
	PKG_CONFIG_PATH="$(FFMPEG_BUILD)/lib/pkgconfig" \
	go build -o subline .

clean:
	rm -rf build/ subline
	cd $(FFMPEG_SRC) && make distclean 2>/dev/null || true
```

**Step 2: Verify Makefile works (partial — just check syntax)**

```bash
make -n all  # dry run to check syntax
```

**Step 3: Commit**

```bash
git add Makefile
git commit -m "build: add Makefile for FFmpeg, whisper.cpp, and Go binary"
```

---

### Task 3: Subtitle formatting — `subtitle.go` + tests

Pure Go, no CGo. This is the easiest starting point and can be tested immediately.

**Files:**
- Create: `subtitle.go`
- Create: `subtitle_test.go`

**Step 1: Write the failing tests**

Port the Python timestamp tests and subtitle writing tests. Reference: Python tests in `TestFormatTimestamp` and `TestTranscribeFile`.

Test cases for `FormatTimestamp`:
- SRT zero → `"00:00:00,000"`
- VTT zero → `"00:00:00.000"`
- SRT uses comma, no dot
- VTT uses dot, no comma
- Large value (10:59:59,999)
- Fractional seconds

Test cases for `WriteSRT` / `WriteVTT`:
- Writes numbered entries with correct timestamp format
- VTT starts with `WEBVTT\n\n`
- Text is trimmed of whitespace

Define a `Segment` struct (to be used later by whisper integration):

```go
type Segment struct {
    Start time.Duration
    End   time.Duration
    Text  string
}
```

**Step 2: Run tests to verify they fail**

```bash
go test -v -run TestFormat
```

**Step 3: Implement `FormatTimestamp` and subtitle writers**

```go
func FormatTimestamp(d time.Duration, format string) string
func WriteSRT(w io.Writer, segments []Segment) error
func WriteVTT(w io.Writer, segments []Segment) error
```

Logic from Python:
- `h, r = divmod(seconds, 3600)` → use `d.Hours()`, `d.Minutes()`, etc.
- SRT: `HH:MM:SS,mmm` (comma before ms)
- VTT: `HH:MM:SS.mmm` (dot before ms)

**Step 4: Run tests to verify they pass**

```bash
go test -v -run TestFormat
go test -v -run TestWrite
```

**Step 5: Commit**

```bash
git add subtitle.go subtitle_test.go
git commit -m "feat: add SRT/VTT subtitle formatting and writing"
```

---

### Task 4: File discovery — `discover.go` + tests

Pure Go. Port the `find_videos` logic.

**Files:**
- Create: `discover.go`
- Create: `discover_test.go`

**Step 1: Write the failing tests**

Port Python's `TestFindVideos`:
- Single file
- Directory with mixed extensions
- Multiple paths
- Nonexistent path (prints warning, returns empty)
- Empty directory
- Audio files (.wav, .flac, .mp3)

**Step 2: Run to verify failure**

```bash
go test -v -run TestFindVideos
```

**Step 3: Implement**

```go
var MediaExts = map[string]bool{
    ".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
    ".webm": true, ".wav": true, ".flac": true, ".mp3": true,
}

func FindMediaFiles(paths []string) []string
```

Use `os.Stat`, `os.ReadDir`, `filepath.Ext`. Same logic as Python version.

**Step 4: Run tests**

```bash
go test -v -run TestFindVideos
```

**Step 5: Commit**

```bash
git add discover.go discover_test.go
git commit -m "feat: add media file discovery"
```

---

### Task 5: Model management — `models.go` + tests

Pure Go (HTTP download, filesystem). No CGo needed.

**Files:**
- Create: `models.go`
- Create: `models_test.go`

**Step 1: Write the failing tests**

Test cases:
- `ModelPath` returns correct cache dir path per OS
- `ModelURL` returns correct Hugging Face URL for each model name
- `ModelURL` maps "turbo" → "large-v3-turbo"
- `ModelURL` maps "large" → "large-v3"
- `EnsureModel` returns cached path when file exists (mock filesystem)
- `EnsureModel` downloads when file missing (mock HTTP)

**Step 2: Run to verify failure**

```bash
go test -v -run TestModel
```

**Step 3: Implement**

```go
// Model name to GGML filename mapping
var modelFiles = map[string]string{
    "tiny":   "ggml-tiny.bin",
    "base":   "ggml-base.bin",
    "small":  "ggml-small.bin",
    "medium": "ggml-medium.bin",
    "turbo":  "ggml-large-v3-turbo.bin",
    "large":  "ggml-large-v3.bin",
}

func CacheDir() string           // ~/.cache/subline/models or ~/Library/Caches/subline/models
func ModelPath(name string) string
func ModelURL(name string) string  // https://huggingface.co/ggerganov/whisper.cpp/resolve/main/<filename>
func EnsureModel(name string) (string, error)  // downloads if missing, returns local path
```

Download with `net/http`, show progress to stderr. Retry once on failure.

**Step 4: Run tests**

```bash
go test -v -run TestModel
```

**Step 5: Commit**

```bash
git add models.go models_test.go
git commit -m "feat: add model download and cache management"
```

---

### Task 6: Build the native dependencies (FFmpeg + whisper.cpp)

This task actually compiles the C libraries. Required before tasks 7 and 8.

**Step 1: Build FFmpeg**

```bash
cd /Users/ellin/code/subline-go
make ffmpeg
```

Verify: `ls build/ffmpeg/lib/libavcodec.a` exists.

**Step 2: Build whisper.cpp**

```bash
make whisper
```

Verify: `ls build/whisper/lib/libwhisper.a` exists.

If the Makefile needs adjustments (library names, paths, cmake flags), fix them now.

**Step 3: Commit any Makefile fixes**

```bash
git add Makefile
git commit -m "fix: adjust Makefile for local build environment"
```

---

### Task 7: Audio pipeline — `audio.go` + tests

CGo via astiav. Requires FFmpeg static libs from Task 6.

**Files:**
- Create: `audio.go`
- Create: `audio_test.go`

**Step 1: Add astiav dependency**

```bash
go get github.com/asticode/go-astiav
```

**Step 2: Write the failing tests**

Test cases using the real test video at `test/videos/fragment.mp4`:
- `ProbeAudioTracks` returns at least one track with stream index and language
- `ExtractAudio` returns `[]float32` with correct sample rate (16000)
- Extracted audio has non-zero samples (not silence)
- Returns error for non-existent file

**Step 3: Run to verify failure**

```bash
CGO_CFLAGS="-I$(pwd)/build/ffmpeg/include" \
CGO_LDFLAGS="-L$(pwd)/build/ffmpeg/lib" \
PKG_CONFIG_PATH="$(pwd)/build/ffmpeg/lib/pkgconfig" \
go test -v -run TestProbe
```

**Step 4: Implement audio probing**

```go
type AudioTrack struct {
    StreamIndex int
    Language    string
    Codec       string
    Channels    int
    SampleRate  int
}

func ProbeAudioTracks(path string) ([]AudioTrack, error)
```

Uses astiav: `AllocFormatContext` → `OpenInput` → `FindStreamInfo` → iterate `Streams()`, filter `MediaTypeAudio`, read `Metadata().Get("language", ...)`.

**Step 5: Implement audio extraction**

```go
func ExtractAudio(path string, streamIndex int) ([]float32, error)
```

Full decode loop:
1. Open input, find stream
2. `FindDecoder` → `AllocCodecContext` → `ToCodecContext` → `Open`
3. `AllocSoftwareResampleContext`
4. Configure destination frame: 16kHz, mono, `SampleFormatFlt`
5. Read packets → `SendPacket` → `ReceiveFrame` → `ConvertFrame` → extract float32 bytes
6. Flush resampler at end
7. Return concatenated `[]float32`

**Step 6: Run tests**

```bash
make subline  # or just the CGo env setup
go test -v -run TestAudio
```

**Step 7: Commit**

```bash
git add audio.go audio_test.go go.mod go.sum
git commit -m "feat: add audio probing and extraction via astiav"
```

---

### Task 8: Whisper integration — `transcribe.go` + tests

CGo via whisper.cpp bindings. Requires whisper.cpp static lib from Task 6.

**Files:**
- Create: `transcribe.go`
- Create: `transcribe_test.go`
- Create: `whisper.go` (CGo bridge to whisper.cpp — may need custom bindings instead of the upstream package)

**Important decision:** The official `ggml-org/whisper.cpp/bindings/go/pkg/whisper` Go package expects to link against a pre-built `libwhisper.a`. We need to write a thin CGo wrapper that includes the whisper.h header directly and links against our locally-built library. This avoids pulling in the upstream Go module's potentially stale CGo configuration.

**Step 1: Write the CGo bridge**

```go
// whisper.go - thin CGo wrapper around whisper.h

package main

/*
#cgo CFLAGS: -I${SRCDIR}/third_party/whisper.cpp/include
#include "whisper.h"
#include <stdlib.h>
*/
import "C"
```

Expose minimal Go API:
- `LoadModel(path string) (*Model, error)`
- `(m *Model) Close()`
- `(m *Model) Transcribe(samples []float32, language string, onProgress func(int)) ([]Segment, error)`

This wraps `whisper_init_from_file`, `whisper_full`, `whisper_full_n_segments`, `whisper_full_get_segment_t0/t1/text`.

**Step 2: Write the failing tests**

Test with the real test video + a small model (tiny). The test should:
- Download the tiny model (75 MB) if not cached
- Extract audio from `test/videos/fragment.mp4`
- Transcribe and get at least 1 segment
- Verify segments have start < end and non-empty text

Mark test as integration test with build tag or `testing.Short()` skip.

**Step 3: Run to verify failure**

```bash
make subline  # ensures libs are built
go test -v -run TestTranscribe -count=1
```

**Step 4: Implement transcription**

The `Transcribe` function:
1. Create `whisper_full_params` with `WHISPER_SAMPLING_GREEDY`
2. Set language (or "auto")
3. Set thread count to `runtime.NumCPU()`
4. Set progress callback (throttled to 1/sec)
5. Call `whisper_full(ctx, params, samples, n_samples)`
6. Iterate segments with `whisper_full_n_segments` / `whisper_full_get_segment_t0/t1/text`
7. Return `[]Segment`

**Step 5: Run tests**

```bash
go test -v -run TestTranscribe -count=1
```

**Step 6: Commit**

```bash
git add whisper.go transcribe.go transcribe_test.go
git commit -m "feat: add whisper.cpp transcription via CGo"
```

---

### Task 9: CLI and main loop — `main.go` + integration test

**Files:**
- Create: `main.go`

**Step 1: Write integration test**

End-to-end test: `subline test/videos/fragment.mp4` should produce `test/videos/fragment.srt` with valid SRT content. Use `os/exec` to run the built binary.

**Step 2: Implement `main.go`**

Port the Python `main()` logic:

```go
func main() {
    // 1. Parse flags (flag package)
    // 2. Find media files
    // 3. Ensure model is downloaded
    // 4. Load whisper model
    // 5. For each file:
    //    a. Pick audio track (probe + select)
    //    b. Extract audio (astiav → []float32)
    //    c. Transcribe (whisper → []Segment)
    //    d. Write subtitle file (SRT/VTT)
    // 6. Print summary
}
```

Flags (matching Python CLI):
- `--language` (string, default "")
- `--model` (string, default "turbo")
- `--audio-track` (int, default -1 = auto)
- `--format` (string, default "srt")
- `--output-dir` (string, default "")
- `--skip-existing` (bool, default false)

Print the banner, file count, model, language, device info same as Python.

**Step 3: Build and test**

```bash
make subline
./subline test/videos/fragment.mp4
# Verify .srt file is created and contains valid subtitles
```

**Step 4: Commit**

```bash
git add main.go
git commit -m "feat: add CLI entry point and main processing loop"
```

---

### Task 10: Progress reporting and polish

**Files:**
- Create: `progress.go`
- Modify: `main.go` (wire up progress)

**Step 1: Implement progress display**

```go
type ProgressReporter struct {
    startTime time.Time
    total     float64  // duration in seconds
}

func (p *ProgressReporter) Update(pct int) {
    // Throttle to 1 update/sec
    // Print: [ 52.3%] 350 segments, 124s elapsed, ETA 1m53s
    // Use \r for in-place updates on TTY
}
```

**Step 2: Wire into transcribe**

Pass `ProgressReporter.Update` as the progress callback to whisper.

**Step 3: Add signal handling**

Trap SIGINT, clean up partial output files.

**Step 4: Test manually with real video**

```bash
make subline
./subline test/videos/fragment.mp4
```

Verify: progress updates display correctly, Ctrl+C cleans up.

**Step 5: Commit**

```bash
git add progress.go main.go
git commit -m "feat: add progress reporting and signal handling"
```

---

### Task 11: Audio track selection logic + tests

**Files:**
- Create: `track.go`
- Create: `track_test.go`

**Step 1: Write failing tests**

Port Python's `TestPickAudioTrack`:
- Manual override returns specified index
- Single track returns that track
- Multiple tracks returns first, prints hint about `--audio-track`
- No tracks returns error

**Step 2: Implement**

```go
func PickAudioTrack(tracks []AudioTrack, manual int) (int, error)
```

**Step 3: Run tests, commit**

```bash
go test -v -run TestPickAudioTrack
git add track.go track_test.go
git commit -m "feat: add audio track selection logic"
```

---

### Task 12: End-to-end integration test

**Files:**
- Create: `integration_test.go`

**Step 1: Write integration test**

Build the binary, run it against `test/videos/fragment.mp4`, verify:
- Exit code 0
- SRT file created next to video
- SRT file has valid format (numbered entries, timestamps, text)
- `--format vtt` produces VTT with WEBVTT header
- `--skip-existing` skips when subtitle already exists
- `--output-dir` writes to specified directory
- Nonexistent file prints error but exits 0 (batch continues)

Use `TestMain` to build the binary once, then run subtests.

**Step 2: Run**

```bash
make subline
go test -v -run TestIntegration -count=1
```

**Step 3: Commit**

```bash
git add integration_test.go
git commit -m "test: add end-to-end integration tests"
```

---

### Task ordering and dependencies

```
Task 1: Scaffolding (no deps)
Task 2: Makefile (depends on 1)
Task 3: Subtitle formatting (depends on 1, pure Go)
Task 4: File discovery (depends on 1, pure Go)
Task 5: Model management (depends on 1, pure Go)
Task 6: Build native deps (depends on 2)
Task 7: Audio pipeline (depends on 6)
Task 8: Whisper integration (depends on 6)
Task 9: CLI main loop (depends on 3, 4, 5, 7, 8)
Task 10: Progress + polish (depends on 9)
Task 11: Track selection (depends on 7, pure Go logic)
Task 12: Integration test (depends on 9)
```

Tasks 3, 4, 5 can be done in parallel (pure Go, no CGo).
Tasks 7 and 8 can be done in parallel after Task 6.
Tasks 10, 11, 12 can be done in parallel after Task 9.
