# subline-go: Design Document

Go rewrite of [subline](../../../subline/) — automatic subtitle generation CLI.
Video in, subtitles out. Single binary, no Python, no external dependencies except a C toolchain at build time.

## Goals

- Single static binary distribution (users download and run)
- Local Whisper transcription via whisper.cpp (no Python, no API calls)
- Metal GPU acceleration on macOS Apple Silicon
- CPU inference on Linux (x86_64, arm64)
- Embed audio extraction (no ffmpeg/ffprobe runtime dependency)
- Same CLI interface as the Python version

## Architecture

### Core dependencies (all via CGo)

| Dependency | Purpose | Integration |
|---|---|---|
| [whisper.cpp](https://github.com/ggml-org/whisper.cpp) | Speech-to-text | git submodule, pre-compiled via CMake, linked statically |
| [astiav](https://github.com/asticode/go-astiav) | Audio extraction + probing | Go bindings for libav/ffmpeg |

### Project structure

```
subline-go/
├── main.go              # CLI entry point (stdlib flag package)
├── transcribe.go        # Whisper model loading + transcription
├── audio.go             # Audio extraction via astiav
├── probe.go             # Audio track detection via astiav
├── subtitle.go          # SRT/VTT formatting and writing
├── progress.go          # Progress reporting (segment count, ETA)
├── models.go            # Model download + cache management
├── Makefile             # Builds libwhisper + go build
├── third_party/
│   └── whisper.cpp/     # git submodule pinned to release tag
└── models/              # .gitignored, local model cache
```

## Build pipeline

The Makefile handles everything:

```makefile
# 1. Build libwhisper.a with platform-appropriate flags
#    macOS: -DGGML_METAL=ON -DGGML_METAL_EMBED_LIBRARY=ON
#    Linux: CPU-only
# 2. Set CGO environment variables
# 3. go build -o subline .
```

Platform detection in the Makefile switches between:
- **macOS:** Metal + Accelerate frameworks, embedded Metal shaders
- **Linux:** CPU-only with OpenBLAS or no BLAS

### CGo flags

```
# macOS
CGO_LDFLAGS: -lwhisper -lggml -lggml-base -lggml-cpu -lm -lstdc++
             -framework Accelerate -framework Metal -framework Foundation -framework CoreGraphics

# Linux
CGO_LDFLAGS: -lwhisper -lggml -lggml-base -lggml-cpu -lm -lstdc++ -lpthread
```

## Model management

### Auto-download on first use

When a model is requested (default: `turbo`), subline checks the cache directory. If the model is missing, it downloads it automatically with a progress bar:

```
$ subline video.mp4
Downloading model 'turbo' (1.6 GB)...
[=============>                    ] 42%
```

No prompts, no confirmation. Subsequent runs use the cached model.

### Cache location

- macOS: `~/Library/Caches/subline/models/`
- Linux: `~/.cache/subline/models/` (XDG_CACHE_HOME)

### Download source

Hugging Face: `https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-{model}.bin`

### Available models

| Model | Size | Notes |
|---|---|---|
| `tiny` | 75 MB | Fastest, lowest accuracy |
| `base` | 142 MB | |
| `small` | 466 MB | Good accuracy/speed balance |
| `medium` | 1.5 GB | |
| `turbo` | 1.6 GB | Default. large-v3-turbo — fast with near-large accuracy |
| `large` | 2.9 GB | Best accuracy, slowest |

## Audio pipeline

Replaces the Python version's `ffmpeg subprocess -> temp WAV file` approach with in-process decoding:

1. **Probe:** Open container with astiav, enumerate audio streams (codec, language, channels)
2. **Decode:** Decode selected audio stream to PCM frames
3. **Resample:** `SwrContext` resamples to 16kHz mono float32 (whisper.cpp's expected input)
4. **Feed:** Pass PCM buffer directly to whisper model in memory

No temp files, no subprocess overhead.

### Supported input formats

Anything libav can decode: `.mp4`, `.mkv`, `.avi`, `.mov`, `.webm`, `.wav`, `.flac`, `.mp3`

## CLI interface

```
subline [flags] <path...>

Flags:
  --language CODE      ISO 639-1 language code (auto-detect if omitted)
  --model NAME         Whisper model: tiny|base|small|medium|turbo|large (default: turbo)
  --audio-track N      Audio stream index (default: first track)
  --format srt|vtt     Output format (default: srt)
  --output-dir DIR     Output directory (default: next to source file)
  --skip-existing      Skip files that already have subtitles
```

### Output format

Identical to the Python version:

```
$ subline --language es lecture.mp4
Found 1 file(s) | model=turbo | language=es | device=metal

Loading model 'turbo'...

[1/1] lecture.mp4
  Using audio track 0 (spa)
  Transcribing 45 min of audio...
  [ 52.3%] 350 segments, 124s elapsed, ETA 1m53s
  Done: 671 segments in 3m58s -> lecture.srt
```

## Progress reporting

Whisper.cpp supports progress callbacks, but benchmarks show they add significant overhead if called too frequently. Strategy:

- Update display at most once per second (wall clock throttle)
- Show: percentage, segment count, elapsed time, ETA
- Use carriage return (`\r`) for in-place updates on TTY

## Error handling

| Scenario | Behavior |
|---|---|
| No audio tracks in file | Print error, skip file, continue batch |
| Model download fails | Retry once, then fail with URL and error message |
| Corrupt/unsupported media | astiav errors on open — skip file, continue |
| Insufficient memory for model | whisper.cpp error — suggest smaller model |
| Ctrl+C during batch | Trap SIGINT, delete partial output, exit cleanly |
| No C compiler at build time | Makefile fails early with clear message |

## CI/CD and distribution

### GitHub Actions

- **macOS runner:** Build with Metal support (ARM64 + x86_64 universal binary if feasible)
- **Linux runner:** Build CPU-only for x86_64 and arm64

### Release artifacts

Upload pre-built binaries to GitHub Releases:
- `subline-darwin-arm64` (macOS Apple Silicon with Metal)
- `subline-darwin-amd64` (macOS Intel with Accelerate)
- `subline-linux-amd64` (CPU)
- `subline-linux-arm64` (CPU)

### Homebrew tap (optional)

```
brew install yourname/tap/subline
```

## Key differences from Python version

| Aspect | Python (subline) | Go (subline-go) |
|---|---|---|
| Whisper engine | faster-whisper (CTranslate2) | whisper.cpp (GGML) |
| Audio extraction | ffmpeg subprocess + temp WAV | astiav in-process, no temp files |
| Distribution | pip install + Python runtime | Single static binary |
| GPU support | CUDA (NVIDIA only) | Metal (macOS), CPU (Linux) |
| Default model | turbo | turbo |
| Runtime deps | Python 3.10+, ffmpeg | None |
| Build deps | None (pip handles it) | C/C++ toolchain, CMake |
