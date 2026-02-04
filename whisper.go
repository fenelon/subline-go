package main

/*
#cgo CFLAGS: -I${SRCDIR}/third_party/whisper.cpp/include -I${SRCDIR}/third_party/whisper.cpp/ggml/include
#include "whisper.h"
#include <stdlib.h>

// CGo trampoline for the progress callback.
// Defined here so CGo can take its address.
extern void goProgressCallback(struct whisper_context * ctx, struct whisper_state * state, int progress, void * user_data);

static void set_progress_callback(struct whisper_full_params *params, void *user_data) {
    params->progress_callback = goProgressCallback;
    params->progress_callback_user_data = user_data;
}
*/
import "C"
import (
	"errors"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

// progressCallbacks maps an opaque ID to a Go callback function.
// This is needed because CGo cannot pass Go function pointers directly to C.
var (
	progressMu        sync.Mutex
	progressCallbacks = map[uintptr]func(int){}
	progressNextID    uintptr
)

func registerProgress(fn func(int)) uintptr {
	progressMu.Lock()
	defer progressMu.Unlock()
	progressNextID++
	id := progressNextID
	progressCallbacks[id] = fn
	return id
}

func unregisterProgress(id uintptr) {
	progressMu.Lock()
	defer progressMu.Unlock()
	delete(progressCallbacks, id)
}

//export goProgressCallback
func goProgressCallback(ctx *C.struct_whisper_context, state *C.struct_whisper_state, progress C.int, userData unsafe.Pointer) {
	id := uintptr(userData)
	progressMu.Lock()
	fn, ok := progressCallbacks[id]
	progressMu.Unlock()
	if ok {
		fn(int(progress))
	}
}

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
// auto-detection. onProgress, if non-nil, is called with percentage [0..100].
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
	if language != "" {
		clang := C.CString(language)
		defer C.free(unsafe.Pointer(clang))
		params.language = clang
	} else {
		// Default params set language to "en". Set to NULL so whisper auto-detects.
		params.language = nil
	}

	// 4. Silence all stdout printing from the C library.
	params.print_progress = C.bool(false)
	params.print_realtime = C.bool(false)
	params.print_special = C.bool(false)
	params.print_timestamps = C.bool(false)

	// 5. Set progress callback if provided.
	var cbID uintptr
	if onProgress != nil {
		cbID = registerProgress(onProgress)
		defer unregisterProgress(cbID)
		C.set_progress_callback(&params, unsafe.Pointer(cbID))
	}

	// 6. Run transcription.
	ret := C.whisper_full(m.ctx, params, (*C.float)(&samples[0]), C.int(len(samples)))
	if ret != 0 {
		return nil, errors.New("whisper transcription failed")
	}

	// 7. Extract segments from the context.
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
