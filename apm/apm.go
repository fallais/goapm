// Package apm is a Go port of WebRTC's Audio Processing Module.
//
// The package surface is stable; module implementations land incrementally.
// Until then, every module is a passthrough — ProcessStream and
// ProcessReverseStream validate the frame and return nil without mutating
// audio. This lets the surrounding test infrastructure compile and run end
// to end before any DSP work begins.
package apm

import "errors"

// ErrFrameShape is returned when a frame's sample rate, channel count, or
// length is inconsistent with the Processor's configuration.
var ErrFrameShape = errors.New("apm: frame shape mismatch")

// Processor is a per-stream APM instance. One Processor is created per
// audio stream and is not safe for concurrent use.
//
// Allocation policy: New does all the allocation for the lifetime of the
// stream. ProcessStream and ProcessReverseStream perform zero allocations
// in the steady state. Tests in test/bench assert this invariant.
type Processor struct {
	cfg Config
}

// New constructs a Processor with the given configuration.
func New(cfg Config) (*Processor, error) {
	return &Processor{cfg: cfg}, nil
}

// Config returns a copy of the Processor's active configuration.
func (p *Processor) Config() Config { return p.cfg }

// ProcessStream processes a single 10 ms near-end (microphone) frame in
// place. The frame is mutated and the same frame is observable after
// return.
//
// Current behavior: passthrough — modules are not yet implemented.
func (p *Processor) ProcessStream(near *Frame) error {
	if err := near.Validate(); err != nil {
		return err
	}
	_ = p.cfg
	return nil
}

// ProcessReverseStream processes a single 10 ms far-end (render) frame.
// This must be called for every render-side frame so the echo canceller
// can track the loudspeaker signal. Frame is read but not mutated in the
// passthrough stub.
func (p *Processor) ProcessReverseStream(far *Frame) error {
	if err := far.Validate(); err != nil {
		return err
	}
	return nil
}

// SetStreamDelayMS reports the round-trip delay between when a far-end
// frame was rendered and when its echo appears in the near-end frame.
// Used by the echo canceller; ignored in the passthrough stub.
func (p *Processor) SetStreamDelayMS(ms int) error {
	if ms < 0 || ms > 500 {
		return errors.New("apm: stream delay out of range [0,500] ms")
	}
	return nil
}
