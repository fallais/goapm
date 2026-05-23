package apm

import "fmt"

// SampleRate is the supported set of stream sample rates, in Hz.
type SampleRate int

const (
	Rate8k  SampleRate = 8000
	Rate16k SampleRate = 16000
	Rate32k SampleRate = 32000
	Rate48k SampleRate = 48000
)

func (r SampleRate) Valid() bool {
	switch r {
	case Rate8k, Rate16k, Rate32k, Rate48k:
		return true
	}
	return false
}

// FrameDurationMS is the fixed processing-block size in milliseconds.
// APM is built around 10 ms frames; all modules assume this.
const FrameDurationMS = 10

// SamplesPerFrame returns the number of samples per channel in one 10 ms
// frame at the given sample rate.
func (r SampleRate) SamplesPerFrame() int {
	return int(r) * FrameDurationMS / 1000
}

// Frame is one 10 ms block of audio, deinterleaved into channels.
//
// Data is indexed [channel][sample]. Each inner slice has length
// SampleRate.SamplesPerFrame(). Frames are processed in place — modules
// mutate Data and return nothing.
type Frame struct {
	SampleRate SampleRate
	Data       [][]float32
}

// NewFrame allocates a zeroed frame for the given rate and channel count.
func NewFrame(rate SampleRate, channels int) *Frame {
	n := rate.SamplesPerFrame()
	data := make([][]float32, channels)
	for i := range data {
		data[i] = make([]float32, n)
	}
	return &Frame{SampleRate: rate, Data: data}
}

// NumChannels returns the channel count.
func (f *Frame) NumChannels() int { return len(f.Data) }

// NumSamples returns the per-channel sample count.
func (f *Frame) NumSamples() int {
	if len(f.Data) == 0 {
		return 0
	}
	return len(f.Data[0])
}

// Validate returns an error if the frame is malformed.
func (f *Frame) Validate() error {
	if !f.SampleRate.Valid() {
		return fmt.Errorf("invalid sample rate %d", f.SampleRate)
	}
	if len(f.Data) == 0 {
		return fmt.Errorf("frame has no channels")
	}
	want := f.SampleRate.SamplesPerFrame()
	for i, ch := range f.Data {
		if len(ch) != want {
			return fmt.Errorf("channel %d has %d samples, want %d", i, len(ch), want)
		}
	}
	return nil
}
