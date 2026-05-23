package main

import (
	"fmt"
	"math/rand/v2"

	"github.com/fallais/gopam/apm"
	"github.com/fallais/gopam/audio/wav"
	"github.com/fallais/gopam/test/metrics"
	"github.com/fallais/gopam/test/synth"
)

// Score is the per-cell measurement bundle written to the JSON report.
// All fields are optional — scenarios omit metrics they don't measure.
type Score struct {
	Cell       Cell    `json:"cell"`
	SegSNRdB   float64 `json:"seg_snr_db,omitempty"`
	SNRGainDB  float64 `json:"snr_gain_db,omitempty"`
	ERLEdB     float64 `json:"erle_db,omitempty"`
	LSDdB      float64 `json:"lsd_db,omitempty"`
	CDdB       float64 `json:"cd_db,omitempty"`
	PEAQLite   float64 `json:"peaq_lite,omitempty"`
	PeakDBFS   float64 `json:"peak_dbfs,omitempty"`
	DurationMS int     `json:"duration_ms,omitempty"`
	Error      string  `json:"error,omitempty"`
}

// scoreCell runs one matrix cell end-to-end: load audio, synthesize the
// test signal as specified by Scenario, process through the apm pipeline,
// compute relevant metrics.
//
// The function is deterministic given a fixed seed so reports are
// reproducible across runs.
func scoreCell(c Cell) Score {
	s := Score{Cell: c}
	clip, rate, err := wav.ReadAll(c.ClipPath)
	if err != nil {
		s.Error = "load clip: " + err.Error()
		return s
	}
	if len(clip) == 0 {
		s.Error = "clip has no channels"
		return s
	}
	clean := clip[0]
	rng := rand.New(rand.NewPCG(uint64(len(c.ClipPath)), 0xCAFE))

	switch c.Scenario {
	case "ns_clean":
		return scoreNSClean(s, clean, rate)
	case "ns_noisy":
		return scoreNSNoisy(s, c, clean, rate, rng)
	case "agc_target":
		return scoreAGC(s, clean, rate)
	case "aec_echo":
		return scoreAEC(s, c, clean, rate, rng)
	default:
		s.Error = fmt.Sprintf("unknown scenario %q", c.Scenario)
	}
	return s
}

func scoreNSClean(s Score, clean []float32, rate int) Score {
	cfg := apm.DefaultConfig()
	cfg.NS = apm.NSConfig{Enabled: true, Level: apm.NSModerate}
	out := process(cfg, apm.SampleRate(rate), clean)
	s.LSDdB = metrics.LogSpectralDistance(clean, out, rate, 512, 256)
	s.CDdB = metrics.CepstralDistance(clean, out, rate, 512, 256, 13)
	s.PEAQLite = metrics.PEAQLite(clean, out, rate)
	s.DurationMS = (len(clean) * 1000) / rate
	return s
}

func scoreNSNoisy(s Score, c Cell, clean []float32, rate int, rng *rand.Rand) Score {
	noiseAll, _, err := wav.ReadAll(c.NoisePath)
	if err != nil {
		s.Error = "load noise: " + err.Error()
		return s
	}
	if len(noiseAll) == 0 {
		s.Error = "noise has no channels"
		return s
	}
	noise := noiseAll[0]
	// Pad/repeat noise to match clean length.
	if len(noise) < len(clean) {
		noise = repeatTo(noise, len(clean))
	} else {
		noise = noise[:len(clean)]
	}
	noisy, err := synth.MixAtSNR(clean, noise, c.SNRDB)
	if err != nil {
		s.Error = "mix: " + err.Error()
		return s
	}
	cfg := apm.DefaultConfig()
	cfg.NS = apm.NSConfig{Enabled: true, Level: apm.NSHigh}
	out := process(cfg, apm.SampleRate(rate), noisy)
	inSNR := metrics.SNR(clean, noisy)
	outSNR := metrics.SNR(clean, out)
	s.SNRGainDB = outSNR - inSNR
	s.LSDdB = metrics.LogSpectralDistance(clean, out, rate, 512, 256)
	s.PEAQLite = metrics.PEAQLite(clean, out, rate)
	s.DurationMS = (len(clean) * 1000) / rate
	_ = rng
	return s
}

func scoreAGC(s Score, clean []float32, rate int) Score {
	synth.ScaleToDBFS(clean, -25) // quiet input
	cfg := apm.DefaultConfig()
	cfg.AGC = apm.AGCConfig{Enabled: true, TargetLevelDBFS: -10, CompressionGain: 9}
	out := process(cfg, apm.SampleRate(rate), clean)
	s.PeakDBFS = metrics.PeakLevelDBFS(out)
	s.DurationMS = (len(clean) * 1000) / rate
	return s
}

func scoreAEC(s Score, c Cell, clean []float32, rate int, rng *rand.Rand) Score {
	var ir []float32
	if c.IRPath != "" {
		ch, _, err := wav.ReadAll(c.IRPath)
		if err != nil {
			s.Error = "load IR: " + err.Error()
			return s
		}
		if len(ch) > 0 {
			ir = ch[0]
		}
	}
	if len(ir) == 0 {
		ir = synth.ExpDecayIR(160, rate, 0.15, rng)
	}
	far := clean // use the clip as far-end
	echo := synth.Convolve(far, ir)[:len(far)]
	mic := append([]float32(nil), echo...) // pure echo, no near-end speech

	cfg := apm.DefaultConfig()
	cfg.AEC.Enabled = true
	p, _ := apm.New(cfg)
	rate2 := apm.SampleRate(rate)
	frame := apm.NewFrame(rate2, 1)
	per := rate2.SamplesPerFrame()

	// Push far-end first, then capture.
	for i := 0; i+per <= len(far); i += per {
		copy(frame.Data[0], far[i:i+per])
		_ = p.ProcessReverseStream(frame)
	}
	out := make([]float32, len(mic))
	for i := 0; i+per <= len(mic); i += per {
		copy(frame.Data[0], mic[i:i+per])
		_ = p.ProcessStream(frame)
		copy(out[i:i+per], frame.Data[0])
	}
	s.ERLEdB = metrics.MaxConvergedERLE(mic, out, per)
	s.DurationMS = (len(mic) * 1000) / rate
	return s
}

func process(cfg apm.Config, rate apm.SampleRate, in []float32) []float32 {
	p, _ := apm.New(cfg)
	frame := apm.NewFrame(rate, 1)
	per := rate.SamplesPerFrame()
	out := make([]float32, (len(in)/per)*per)
	for i := 0; i+per <= len(in); i += per {
		copy(frame.Data[0], in[i:i+per])
		_ = p.ProcessStream(frame)
		copy(out[i:i+per], frame.Data[0])
	}
	return out
}

func repeatTo(x []float32, n int) []float32 {
	if len(x) == 0 {
		return make([]float32, n)
	}
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		out[i] = x[i%len(x)]
	}
	return out
}
