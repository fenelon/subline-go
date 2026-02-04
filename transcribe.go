package main

/*
#cgo CFLAGS: -I${SRCDIR}/third_party/whisper.cpp/include -I${SRCDIR}/third_party/whisper.cpp/ggml/include
#include "whisper.h"
*/
import "C"

// DetectedLanguage returns the ISO-639-1 code of the language detected (or
// used) during the most recent Transcribe call.  It reads the language id
// stored in the context's default state, so it must be called after
// Transcribe.
func (m *WhisperModel) DetectedLanguage() string {
	if m.ctx == nil {
		return ""
	}
	id := C.whisper_full_lang_id(m.ctx)
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
