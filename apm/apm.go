// Package apm is a Go port of WebRTC's Audio Processing Module: a
// per-stream pipeline of high-pass filter, acoustic echo cancellation,
// noise suppression, and automatic gain control.
package apm

import (
	"errors"
	"fmt"

	"github.com/fallais/gopam/agc"
	"github.com/fallais/gopam/hpf"
	"github.com/fallais/gopam/ns"
)

// ErrFrameShape is returned when a frame's shape mismatches the
// Processor's configured rate or channel count.
var ErrFrameShape = errors.New("apm: frame shape mismatch")

// Processor is a per-stream APM instance. One Processor per audio stream.
// Not safe for concurrent use.
type Processor struct {
	cfg Config

	hpf *hpf.Filter
	ns  *ns.Suppressor
	agc *agc.Controller
}

// New constructs a Processor.
func New(cfg Config) (*Processor, error) {
	if cfg.NumChannels <= 0 {
		return nil, fmt.Errorf("apm: NumChannels must be > 0, got %d", cfg.NumChannels)
	}
	if !cfg.SampleRate.Valid() {
		return nil, fmt.Errorf("apm: invalid SampleRate %d", cfg.SampleRate)
	}
	if cfg.ReverseSampleRate == 0 {
		cfg.ReverseSampleRate = cfg.SampleRate
	}
	if cfg.ReverseNumChannels == 0 {
		cfg.ReverseNumChannels = cfg.NumChannels
	}

	p := &Processor{cfg: cfg}

	if cfg.HPF.Enabled {
		f, err := hpf.New(int(cfg.SampleRate), cfg.NumChannels)
		if err != nil {
			return nil, fmt.Errorf("apm: hpf: %w", err)
		}
		p.hpf = f
	}

	if cfg.NS.Enabled {
		s, err := ns.NewSuppressor(ns.Config{TargetLevel: mapNSLevel(cfg.NS.Level)}, int(cfg.SampleRate), cfg.NumChannels)
		if err != nil {
			return nil, fmt.Errorf("apm: ns: %w", err)
		}
		p.ns = s
	}

	if cfg.AGC.Enabled {
		c, err := agc.New(agc.Config{FixedDigitalGainDB: cfg.AGC.CompressionGain}, int(cfg.SampleRate), cfg.NumChannels)
		if err != nil {
			return nil, fmt.Errorf("apm: agc: %w", err)
		}
		p.agc = c
	}

	return p, nil
}

// Config returns a copy of the active configuration.
func (p *Processor) Config() Config { return p.cfg }

// ProcessStream processes one 10 ms near-end frame in place.
//
// Public API takes samples in the [-1, +1] convention. Internally the
// pipeline runs at WebRTC's [-32768, +32767] scale (the upstream modules
// are calibrated for that range). Conversion happens at the boundary.
func (p *Processor) ProcessStream(near *Frame) error {
	if err := p.validateFrame(near, p.cfg.SampleRate, p.cfg.NumChannels); err != nil {
		return err
	}
	scaleUp(near.Data)
	if p.hpf != nil {
		if err := p.hpf.Process(near.Data); err != nil {
			scaleDown(near.Data)
			return err
		}
	}
	if p.ns != nil {
		if err := p.ns.Analyze(near.Data); err != nil {
			scaleDown(near.Data)
			return err
		}
		if err := p.ns.Process(near.Data); err != nil {
			scaleDown(near.Data)
			return err
		}
	}
	if p.agc != nil {
		if err := p.agc.Process(near.Data); err != nil {
			scaleDown(near.Data)
			return err
		}
	}
	scaleDown(near.Data)
	return nil
}

const apmScale = 32768.0

func scaleUp(data [][]float32) {
	for c := range data {
		for i := range data[c] {
			data[c][i] *= apmScale
		}
	}
}

func scaleDown(data [][]float32) {
	const inv = 1.0 / apmScale
	for c := range data {
		for i := range data[c] {
			data[c][i] *= inv
		}
	}
}

// ProcessReverseStream processes one 10 ms far-end (render) frame. The
// echo canceller consumes this stream; other modules ignore it.
func (p *Processor) ProcessReverseStream(far *Frame) error {
	return p.validateFrame(far, p.cfg.ReverseSampleRate, p.cfg.ReverseNumChannels)
}

// SetStreamDelayMS reports round-trip stream delay (ms) to the echo canceller.
func (p *Processor) SetStreamDelayMS(ms int) error {
	if ms < 0 || ms > 500 {
		return fmt.Errorf("apm: stream delay %d out of [0, 500]", ms)
	}
	return nil
}

func (p *Processor) validateFrame(f *Frame, wantRate SampleRate, wantCh int) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if f.SampleRate != wantRate {
		return fmt.Errorf("%w: rate %d, want %d", ErrFrameShape, f.SampleRate, wantRate)
	}
	if f.NumChannels() != wantCh {
		return fmt.Errorf("%w: channels %d, want %d", ErrFrameShape, f.NumChannels(), wantCh)
	}
	return nil
}

func mapNSLevel(l NSLevel) ns.SuppressionLevel {
	switch l {
	case NSLow:
		return ns.Level6dB
	case NSModerate:
		return ns.Level12dB
	case NSHigh:
		return ns.Level18dB
	case NSVeryHigh:
		return ns.Level21dB
	default:
		return ns.Level12dB
	}
}
