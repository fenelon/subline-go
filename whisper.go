package main

/*
#cgo CFLAGS: -I${SRCDIR}/third_party/whisper.cpp/include -I${SRCDIR}/third_party/whisper.cpp/ggml/include
#include "whisper.h"
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"runtime"
	"time"
	"unsafe"
)

// WhisperModel wraps a whisper.cpp context loaded from a GGML model file.
// It is NOT safe for concurrent use; callers must serialize access or use
// separate model instances.
type WhisperModel struct {
	ctx *C.struct_whisper_context
}

// LoadModel loads a GGML whisper model from disk and returns a WhisperModel.
// The caller must call Close() when done to free the underlying C resources.
func LoadModel(path string) (*WhisperModel, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	ctx := C.whisper_init_from_file_with_params(cpath, C.whisper_context_default_params())
	if ctx == nil {
		return nil, errors.New("failed to load whisper model: " + path)
	}
	return &WhisperModel{ctx: ctx}, nil
}

// Close frees all C resources associated with the model.
// It is safe to call Close multiple times.
func (m *WhisperModel) Close() {
	if m.ctx != nil {
		C.whisper_free(m.ctx)
		m.ctx = nil
	}
}

// Transcribe runs whisper inference on 16 kHz float32 PCM samples and returns
// timestamped text segments.
//
// language should be an ISO-639-1 code (e.g. "en", "de") or "" for
// auto-detection.  onProgress, if non-nil, is currently reserved for future
// use (progress callbacks require CGo function pointer trampolines).
func (m *WhisperModel) Transcribe(samples []float32, language string, onProgress func(int)) ([]Segment, error) {
	if m.ctx == nil {
		return nil, errors.New("whisper model is closed")
	}
	if len(samples) == 0 {
		return nil, errors.New("no audio samples provided")
	}

	// 1. Create default params with greedy sampling strategy.
	params := C.whisper_full_default_params(C.WHISPER_SAMPLING_GREEDY)

	// 2. Thread count: use all available CPUs.
	params.n_threads = C.int(runtime.NumCPU())

	// 3. Language setting.
	var clang *C.char
	if language != "" {
		clang = C.CString(language)
		defer C.free(unsafe.Pointer(clang))
		params.language = clang
	}
	// If language is empty, whisper auto-detects (default behavior).

	// 4. Silence all stdout printing from the C library.
	params.print_progress = C.bool(false)
	params.print_realtime = C.bool(false)
	params.print_special = C.bool(false)
	params.print_timestamps = C.bool(false)

	// 5. Run transcription.
	ret := C.whisper_full(m.ctx, params, (*C.float)(&samples[0]), C.int(len(samples)))
	if ret != 0 {
		return nil, errors.New("whisper transcription failed")
	}

	// 6. Extract segments from the context.
	nSegments := int(C.whisper_full_n_segments(m.ctx))
	segments := make([]Segment, 0, nSegments)
	for i := 0; i < nSegments; i++ {
		ci := C.int(i)
		t0 := int64(C.whisper_full_get_segment_t0(m.ctx, ci)) // centiseconds (10 ms units)
		t1 := int64(C.whisper_full_get_segment_t1(m.ctx, ci))
		text := C.GoString(C.whisper_full_get_segment_text(m.ctx, ci))

		segments = append(segments, Segment{
			Start: time.Duration(t0) * 10 * time.Millisecond,
			End:   time.Duration(t1) * 10 * time.Millisecond,
			Text:  text,
		})
	}

	return segments, nil
}
