# Makefile for subline — builds FFmpeg static libs, whisper.cpp static lib, then Go binary.
#
# Targets:
#   all      — build everything (ffmpeg, whisper, subline)
#   ffmpeg   — build FFmpeg static libraries
#   whisper  — build whisper.cpp static library (+ ggml)
#   subline  — build the Go binary
#   clean    — remove all build artifacts

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------
ROOT_DIR   := $(shell pwd)
BUILD_DIR  := $(ROOT_DIR)/build

FFMPEG_SRC := $(ROOT_DIR)/third_party/FFmpeg
FFMPEG_BUILD := $(BUILD_DIR)/ffmpeg
FFMPEG_PREFIX := $(FFMPEG_BUILD)/install

WHISPER_SRC := $(ROOT_DIR)/third_party/whisper.cpp
WHISPER_BUILD := $(BUILD_DIR)/whisper

# ---------------------------------------------------------------------------
# Platform detection
# ---------------------------------------------------------------------------
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Darwin)
  # macOS ----------------------------------------------------------------
  FFMPEG_PLATFORM_FLAGS := --enable-videotoolbox --enable-audiotoolbox
  WHISPER_PLATFORM_FLAGS := -DGGML_METAL=ON -DGGML_METAL_EMBED_LIBRARY=ON
  PLATFORM_LDFLAGS := -framework Accelerate -framework Metal -framework Foundation -framework CoreGraphics
  # On Apple Silicon, ggml-metal is built; on Linux it is not.
  WHISPER_METAL_LIB = $(WHISPER_BUILD)/ggml/src/ggml-metal/libggml-metal.a
else
  # Linux ----------------------------------------------------------------
  FFMPEG_PLATFORM_FLAGS :=
  WHISPER_PLATFORM_FLAGS := -DGGML_METAL=OFF
  PLATFORM_LDFLAGS :=
  WHISPER_METAL_LIB =
endif

# Number of parallel jobs for sub-builds
NPROC := $(shell nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)

# ---------------------------------------------------------------------------
# FFmpeg static libraries we need
# ---------------------------------------------------------------------------
FFMPEG_LIBS := \
  $(FFMPEG_PREFIX)/lib/libavcodec.a \
  $(FFMPEG_PREFIX)/lib/libavformat.a \
  $(FFMPEG_PREFIX)/lib/libswresample.a \
  $(FFMPEG_PREFIX)/lib/libavutil.a \
  $(FFMPEG_PREFIX)/lib/libavfilter.a

# ---------------------------------------------------------------------------
# Whisper / ggml static libraries
# ---------------------------------------------------------------------------
WHISPER_LIBS := \
  $(WHISPER_BUILD)/src/libwhisper.a \
  $(WHISPER_BUILD)/ggml/src/libggml.a \
  $(WHISPER_BUILD)/ggml/src/libggml-base.a \
  $(WHISPER_BUILD)/ggml/src/libggml-cpu.a \
  $(WHISPER_METAL_LIB)

# ---------------------------------------------------------------------------
# CGo flags for the Go build
# ---------------------------------------------------------------------------
CGO_CFLAGS := \
  -I$(FFMPEG_PREFIX)/include \
  -I$(WHISPER_SRC)/include \
  -I$(WHISPER_SRC)/ggml/include

CGO_LDFLAGS := \
  -L$(FFMPEG_PREFIX)/lib \
  -L$(WHISPER_BUILD)/src \
  -L$(WHISPER_BUILD)/ggml/src \
  -lavformat -lavcodec -lswresample -lavfilter -lavutil \
  -lwhisper -lggml -lggml-base -lggml-cpu \
  -L$(WHISPER_BUILD)/ggml/src/ggml-blas -lggml-blas \
  $(if $(WHISPER_METAL_LIB),-L$(WHISPER_BUILD)/ggml/src/ggml-metal -lggml-metal) \
  $(PLATFORM_LDFLAGS) \
  -lstdc++ -lm -lpthread

PKG_CONFIG_PATH := $(FFMPEG_PREFIX)/lib/pkgconfig

# ===================================================================
# TARGETS
# ===================================================================

.PHONY: all ffmpeg whisper subline clean

all: subline

# -------------------------------------------------------------------
# FFmpeg
# -------------------------------------------------------------------
ffmpeg: $(FFMPEG_LIBS)

$(FFMPEG_LIBS): $(FFMPEG_BUILD)/.built

$(FFMPEG_BUILD)/.built: $(FFMPEG_SRC)/configure
	@echo "==> Configuring FFmpeg..."
	@mkdir -p $(FFMPEG_BUILD)
	cd $(FFMPEG_BUILD) && $(FFMPEG_SRC)/configure \
		--prefix=$(FFMPEG_PREFIX) \
		--disable-everything \
		--disable-programs \
		--disable-doc \
		--disable-network \
		--disable-autodetect \
		--disable-shared \
		--enable-static \
		--enable-gpl \
		--enable-swresample \
		--enable-avfilter \
		--enable-demuxer=mov,matroska,avi,wav,flac,mp3 \
		--enable-decoder=aac,mp3,flac,opus,vorbis,pcm_s16le,pcm_f32le,ac3,eac3,dts \
		--enable-parser=aac,mpegaudio,flac,opus,vorbis,ac3,dts \
		--enable-protocol=file \
		--enable-filter=aresample \
		$(FFMPEG_PLATFORM_FLAGS) \
		--extra-cflags="-fPIC" \
		--extra-ldflags="-fPIC"
	@echo "==> Building FFmpeg..."
	cd $(FFMPEG_BUILD) && $(MAKE) -j$(NPROC)
	@echo "==> Installing FFmpeg..."
	cd $(FFMPEG_BUILD) && $(MAKE) install
	@touch $@

# -------------------------------------------------------------------
# whisper.cpp
# -------------------------------------------------------------------
whisper: $(WHISPER_BUILD)/src/libwhisper.a

$(WHISPER_BUILD)/src/libwhisper.a: $(WHISPER_BUILD)/.built

$(WHISPER_BUILD)/.built: $(WHISPER_SRC)/CMakeLists.txt
	@echo "==> Configuring whisper.cpp..."
	@mkdir -p $(WHISPER_BUILD)
	cd $(WHISPER_BUILD) && cmake $(WHISPER_SRC) \
		-DCMAKE_BUILD_TYPE=Release \
		-DBUILD_SHARED_LIBS=OFF \
		-DWHISPER_BUILD_EXAMPLES=OFF \
		-DWHISPER_BUILD_TESTS=OFF \
		-DWHISPER_BUILD_SERVER=OFF \
		-DGGML_CPU=ON \
		$(WHISPER_PLATFORM_FLAGS)
	@echo "==> Building whisper.cpp..."
	cmake --build $(WHISPER_BUILD) --config Release -j$(NPROC)
	@touch $@

# -------------------------------------------------------------------
# Go binary
# -------------------------------------------------------------------
subline: ffmpeg whisper
	@echo "==> Building subline Go binary..."
	CGO_ENABLED=1 \
	CGO_CFLAGS="$(CGO_CFLAGS)" \
	CGO_LDFLAGS="$(CGO_LDFLAGS)" \
	PKG_CONFIG_PATH="$(PKG_CONFIG_PATH)" \
	go build -o $(ROOT_DIR)/subline .

# -------------------------------------------------------------------
# Clean
# -------------------------------------------------------------------
clean:
	@echo "==> Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f $(ROOT_DIR)/subline
