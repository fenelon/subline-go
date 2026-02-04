package main

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/asticode/go-astiav"
)

// AudioTrack holds metadata for a single audio stream in a media file.
type AudioTrack struct {
	StreamIndex int
	Language    string
	Codec       string
	Channels    int
	SampleRate  int
}

// ProbeAudioTracks opens a media file and returns metadata for each audio stream.
func ProbeAudioTracks(path string) ([]AudioTrack, error) {
	fc := astiav.AllocFormatContext()
	if fc == nil {
		return nil, fmt.Errorf("allocating format context")
	}
	defer fc.CloseInput()

	if err := fc.OpenInput(path, nil, nil); err != nil {
		return nil, fmt.Errorf("opening %q: %w", path, err)
	}

	if err := fc.FindStreamInfo(nil); err != nil {
		return nil, fmt.Errorf("finding stream info: %w", err)
	}

	var tracks []AudioTrack
	for _, s := range fc.Streams() {
		cp := s.CodecParameters()
		if cp.MediaType() != astiav.MediaTypeAudio {
			continue
		}

		lang := ""
		if md := s.Metadata(); md != nil {
			if entry := md.Get("language", nil, 0); entry != nil {
				lang = entry.Value()
			}
		}

		tracks = append(tracks, AudioTrack{
			StreamIndex: s.Index(),
			Language:    lang,
			Codec:       cp.CodecID().Name(),
			Channels:    cp.ChannelLayout().Channels(),
			SampleRate:  cp.SampleRate(),
		})
	}

	return tracks, nil
}

// ExtractAudio decodes the specified audio stream and resamples it to
// 16 kHz mono float32, suitable for speech recognition models.
func ExtractAudio(path string, streamIndex int) ([]float32, error) {
	// Open input
	fc := astiav.AllocFormatContext()
	if fc == nil {
		return nil, fmt.Errorf("allocating format context")
	}
	defer fc.CloseInput()

	if err := fc.OpenInput(path, nil, nil); err != nil {
		return nil, fmt.Errorf("opening %q: %w", path, err)
	}

	if err := fc.FindStreamInfo(nil); err != nil {
		return nil, fmt.Errorf("finding stream info: %w", err)
	}

	// Find the requested audio stream
	var audioStream *astiav.Stream
	for _, s := range fc.Streams() {
		if s.Index() == streamIndex {
			audioStream = s
			break
		}
	}
	if audioStream == nil {
		return nil, fmt.Errorf("stream index %d not found", streamIndex)
	}
	if audioStream.CodecParameters().MediaType() != astiav.MediaTypeAudio {
		return nil, fmt.Errorf("stream %d is not an audio stream", streamIndex)
	}

	// Set up decoder
	codecParams := audioStream.CodecParameters()
	codec := astiav.FindDecoder(codecParams.CodecID())
	if codec == nil {
		return nil, fmt.Errorf("decoder not found for codec %s", codecParams.CodecID().Name())
	}

	cc := astiav.AllocCodecContext(codec)
	if cc == nil {
		return nil, fmt.Errorf("allocating codec context")
	}
	defer cc.Free()

	if err := codecParams.ToCodecContext(cc); err != nil {
		return nil, fmt.Errorf("copying codec parameters: %w", err)
	}

	if err := cc.Open(codec, nil); err != nil {
		return nil, fmt.Errorf("opening codec: %w", err)
	}

	// Set up software resampler
	swrCtx := astiav.AllocSoftwareResampleContext()
	if swrCtx == nil {
		return nil, fmt.Errorf("allocating software resample context")
	}
	defer swrCtx.Free()

	// Allocate frames and packet
	srcFrame := astiav.AllocFrame()
	if srcFrame == nil {
		return nil, fmt.Errorf("allocating source frame")
	}
	defer srcFrame.Free()

	dstFrame := astiav.AllocFrame()
	if dstFrame == nil {
		return nil, fmt.Errorf("allocating destination frame")
	}
	defer dstFrame.Free()

	// prepareDst resets the destination frame and re-applies the output
	// format so that ConvertFrame sees a clean frame each time.
	prepareDst := func() {
		dstFrame.Unref()
		dstFrame.SetSampleRate(16000)
		dstFrame.SetSampleFormat(astiav.SampleFormatFlt)
		dstFrame.SetChannelLayout(astiav.ChannelLayoutMono)
	}

	// Configure destination frame for 16kHz mono float32
	prepareDst()

	pkt := astiav.AllocPacket()
	if pkt == nil {
		return nil, fmt.Errorf("allocating packet")
	}
	defer pkt.Free()

	// resampleAndCollect resamples a decoded frame (or nil to flush) and
	// appends the resulting float32 samples to the output slice.
	resampleAndCollect := func(src *astiav.Frame, samples *[]float32) error {
		prepareDst()
		if err := swrCtx.ConvertFrame(src, dstFrame); err != nil {
			return err
		}
		if dstFrame.NbSamples() == 0 {
			return nil
		}
		floats, err := extractFloat32Samples(dstFrame)
		if err != nil {
			return err
		}
		*samples = append(*samples, floats...)
		return nil
	}

	// Decode and resample loop
	var samples []float32

	for {
		err := fc.ReadFrame(pkt)
		if err != nil {
			if errors.Is(err, astiav.ErrEof) {
				break
			}
			return nil, fmt.Errorf("reading frame: %w", err)
		}

		// Skip packets from other streams
		if pkt.StreamIndex() != streamIndex {
			pkt.Unref()
			continue
		}

		if err := cc.SendPacket(pkt); err != nil {
			pkt.Unref()
			return nil, fmt.Errorf("sending packet to decoder: %w", err)
		}
		pkt.Unref()

		// Receive all decoded frames
		for {
			err := cc.ReceiveFrame(srcFrame)
			if err != nil {
				if errors.Is(err, astiav.ErrEagain) {
					break
				}
				if errors.Is(err, astiav.ErrEof) {
					break
				}
				return nil, fmt.Errorf("receiving frame: %w", err)
			}

			if err := resampleAndCollect(srcFrame, &samples); err != nil {
				srcFrame.Unref()
				return nil, fmt.Errorf("converting frame: %w", err)
			}
			srcFrame.Unref()
		}
	}

	// Flush the decoder
	if err := cc.SendPacket(nil); err != nil && !errors.Is(err, astiav.ErrEof) {
		return nil, fmt.Errorf("flushing decoder: %w", err)
	}
	for {
		err := cc.ReceiveFrame(srcFrame)
		if err != nil {
			if errors.Is(err, astiav.ErrEagain) || errors.Is(err, astiav.ErrEof) {
				break
			}
			return nil, fmt.Errorf("receiving frame during flush: %w", err)
		}

		if err := resampleAndCollect(srcFrame, &samples); err != nil {
			srcFrame.Unref()
			return nil, fmt.Errorf("converting frame during flush: %w", err)
		}
		srcFrame.Unref()
	}

	// Flush the resampler (drain buffered samples)
	prepareDst()
	if err := swrCtx.ConvertFrame(nil, dstFrame); err != nil {
		if !errors.Is(err, astiav.ErrEof) {
			return nil, fmt.Errorf("flushing resampler: %w", err)
		}
	} else if dstFrame.NbSamples() > 0 {
		floats, err := extractFloat32Samples(dstFrame)
		if err != nil {
			return nil, fmt.Errorf("extracting flushed samples: %w", err)
		}
		samples = append(samples, floats...)
	}

	return samples, nil
}

// extractFloat32Samples reads interleaved float32 data from a resampled frame.
func extractFloat32Samples(frame *astiav.Frame) ([]float32, error) {
	nbSamples := frame.NbSamples()
	if nbSamples == 0 {
		return nil, nil
	}

	// For SampleFormatFlt (interleaved float32), mono: nbSamples * 1 channel * 4 bytes
	bufSize, err := frame.SamplesBufferSize(1)
	if err != nil {
		return nil, fmt.Errorf("getting samples buffer size: %w", err)
	}

	buf := make([]byte, bufSize)
	if _, err := frame.SamplesCopyToBuffer(buf, 1); err != nil {
		return nil, fmt.Errorf("copying samples to buffer: %w", err)
	}

	// Convert []byte to []float32 using unsafe
	nFloats := len(buf) / 4
	floats := unsafe.Slice((*float32)(unsafe.Pointer(&buf[0])), nFloats)

	// Copy into a new slice so the result does not alias buf
	out := make([]float32, nFloats)
	copy(out, floats)
	return out, nil
}
