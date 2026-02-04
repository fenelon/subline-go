package main

/*
#cgo CFLAGS: -I${SRCDIR}/third_party/whisper.cpp/include -I${SRCDIR}/third_party/whisper.cpp/ggml/include
#include "whisper.h"
*/
import "C"
import "runtime"

// DetectLanguage analyses the first 30 seconds of audio and returns the
// ISO-639-1 code of the most likely language (e.g. "en", "ru").
// This is a lightweight operation compared to full transcription.
func (m *WhisperModel) DetectLanguage(samples []float32) string {
	if m.ctx == nil || len(samples) == 0 {
		return ""
	}

	// whisper_pcm_to_mel computes the mel spectrogram (uses first 30s).
	ret := C.whisper_pcm_to_mel(m.ctx, (*C.float)(&samples[0]), C.int(len(samples)), C.int(runtime.NumCPU()))
	if ret != 0 {
		return ""
	}

	// whisper_lang_auto_detect returns the top language id.
	id := C.whisper_lang_auto_detect(m.ctx, 0, C.int(runtime.NumCPU()), nil)
	if id < 0 {
		return ""
	}
	return C.GoString(C.whisper_lang_str(id))
}

// IsMultilingual reports whether the loaded model supports multiple
// languages.  Monolingual models (e.g. *.en) only support English.
func (m *WhisperModel) IsMultilingual() bool {
	if m.ctx == nil {
		return false
	}
	return C.whisper_is_multilingual(m.ctx) != 0
}
