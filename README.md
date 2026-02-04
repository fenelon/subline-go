<div align="center">

# SUBLINE
AI subtitles made easy

**Generate subtitles for any video or audio file, right from your terminal.**

Built on [whisper.cpp](https://github.com/ggerganov/whisper.cpp) with Metal GPU acceleration on macOS.

99 languages &bull; auto-detection &bull; SRT & VTT &bull; multi-track audio &bull; batch processing

<br>

<img width="912" height="744" alt="SCR-20260204-tzho" src="https://github.com/user-attachments/assets/e8fdf01e-496a-4e31-9df7-e0d29827e3ae" />

</div>

<br>

## Quick start

```bash
git clone --recursive https://github.com/fenelon/subline-go && cd subline-go
make
```

```bash
./subline movie.mp4
```

That's it. The model downloads automatically on first run, language is auto-detected, and the subtitle file lands next to the source.

## Usage

```
subline [options] <path...>
```

Pass files or directories &mdash; Subline finds all media files and processes them in order.

### Options

| Flag | Values | Description | Default |
|------|--------|-------------|---------|
| `-l, --language` | `en`, `ru`, `de`, `fr`, ... ([ISO 639-1](https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes)) | Language code | auto-detect |
| `-m, --model` | `tiny`, `base`, `small`, `medium`, `turbo`, `large` | Whisper model | `turbo` |
| `-a, --audio-track` | `0`, `1`, `2`, ... | Audio stream index | auto-detect |
| `-f, --format` | `srt`, `vtt` | Subtitle format | `srt` |
| `-o, --output-dir` | path | Output directory | next to source |
| `-s, --skip-existing` | | Skip already-subtitled files | off |
| `-v, --verbose` | | Show whisper.cpp engine output | off |

### Examples

```bash
# Specify language and model
subline -l ru -m large movie.mkv

# Process a whole directory, output as VTT
subline -f vtt -o ./subs/ ~/Movies/

# Re-run without re-processing existing files
subline -s ~/Movies/
```

## Models

Models download automatically to `~/Library/Caches/subline/models` (macOS) or `~/.cache/subline/models` (Linux) on first use.

| Model | Size | Speed | Quality |
|-------|------|-------|---------|
| `tiny` | 75 MB | Fastest | Low |
| `base` | 142 MB | Fast | Fair |
| `small` | 466 MB | Moderate | Good |
| `medium` | 1.5 GB | Slow | Great |
| **`turbo`** | **1.6 GB** | **Fast** | **Great (default)** |
| `large` | 3.1 GB | Slowest | Best |

## Supported formats

| | Formats |
|---|---------|
| **Video** | MP4, MKV, AVI, MOV, WebM |
| **Audio** | WAV, FLAC, MP3 |
| **Codecs** | AAC, MP3, FLAC, Opus, Vorbis, AC3, E-AC3, DTS, PCM |

## Building from source

**Requirements:** Go 1.22+, CMake, C/C++ compiler.

FFmpeg and whisper.cpp are vendored as submodules and built as static libraries &mdash; no system dependencies needed.

```bash
git clone --recursive https://github.com/fenelon/subline-go
cd subline-go
make        # builds ffmpeg, whisper.cpp, then the Go binary
./subline --help
```

## License

MIT
