// Package agc is a partial Go port of WebRTC's GainController2: it
// implements the fixed-digital-gain + limiter chain (gain_applier and
// limiter from modules/audio_processing/agc2/). The adaptive-digital
// path (VAD-driven gain adaptation) is not yet ported.
//
// Inputs are expected at WebRTC's float-int16 scale (samples in
// [-32768, 32767]).
package agc

import (
	"fmt"
	"math"
)

// Config configures a Controller.
type Config struct {
	// FixedDigitalGainDB is applied uniformly to every sample.
	FixedDigitalGainDB float32
}

// Controller composes a fixed-digital GainApplier with a Limiter.
type Controller struct {
	sampleRate      int
	numChannels     int
	samplesPerFrame int
	gain            *gainApplier
	limiter         *limiter
}

// New constructs a Controller for the given stream parameters.
func New(cfg Config, sampleRateHz, numChannels int) (*Controller, error) {
	if numChannels <= 0 {
		return nil, fmt.Errorf("agc: numChannels must be > 0")
	}
	if sampleRateHz <= 0 {
		return nil, fmt.Errorf("agc: sampleRateHz must be > 0")
	}
	samplesPerFrame := sampleRateHz * kFrameDurationMs / 1000
	if samplesPerFrame > kMaxSamplesPerChannel {
		return nil, fmt.Errorf("agc: samplesPerFrame %d exceeds max %d", samplesPerFrame, kMaxSamplesPerChannel)
	}
	if samplesPerFrame%kSubFramesInFrame != 0 {
		return nil, fmt.Errorf("agc: samplesPerFrame %d not divisible by %d sub-frames", samplesPerFrame, kSubFramesInFrame)
	}
	return &Controller{
		sampleRate:      sampleRateHz,
		numChannels:     numChannels,
		samplesPerFrame: samplesPerFrame,
		gain:            newGainApplier(false, dbToRatio(cfg.FixedDigitalGainDB)),
		limiter:         newLimiter(samplesPerFrame),
	}, nil
}

// SetFixedGainDB changes the fixed digital gain in dB. The next frame
// applies a smooth ramp from the previous gain.
func (c *Controller) SetFixedGainDB(db float32) {
	c.gain.setGainFactor(dbToRatio(db))
}

// Process applies fixed gain + limiter to one 10 ms frame in place.
func (c *Controller) Process(channels [][]float32) error {
	if len(channels) != c.numChannels {
		return fmt.Errorf("agc: got %d channels, want %d", len(channels), c.numChannels)
	}
	for i, ch := range channels {
		if len(ch) != c.samplesPerFrame {
			return fmt.Errorf("agc: channel %d has %d samples, want %d", i, len(ch), c.samplesPerFrame)
		}
	}
	c.gain.apply(channels)
	c.limiter.process(channels)
	return nil
}

// Reset clears the limiter state.
func (c *Controller) Reset() { c.limiter.reset() }

func dbToRatio(db float32) float32 {
	return float32(math.Pow(10, float64(db)/20))
}
