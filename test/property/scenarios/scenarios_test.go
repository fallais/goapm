package scenarios

import (
	"math/rand/v2"
	"testing"

	"github.com/fallais/gopam/apm"
	"github.com/fallais/gopam/test/metrics"
	"github.com/fallais/gopam/test/property"
	"github.com/fallais/gopam/test/synth"
)

// --- AEC: ERLE on a synthetic echo path ----------------------------------

func TestAEC_ERLE(t *testing.T) {
	property.RequireImplemented(t, property.ModuleAEC)
	bounds := property.MustBounds(t, "aec_erle")

	const rate = 16000
	const dur = rate * 3 // 3 s
	rng := rand.New(rand.NewPCG(1, 1))
	far := synth.SpeechShapedNoise(dur, rate, rng)
	ir := synth.ExpDecayIR(160, rate, 0.15, rng)
	echo := synth.Convolve(far, ir)[:dur]
	mic := make([]float32, dur) // no near-end speech: pure echo
	for i := range mic {
		mic[i] = echo[i]
	}

	cfg := apm.DefaultConfig()
	cfg.AEC.Enabled = true
	p, _ := apm.New(cfg)
	pipe := property.NewPipeline(p, apm.Rate16k)

	if err := pipe.ProcessReverseStream(far); err != nil {
		t.Fatal(err)
	}
	out := make([]float32, dur)
	if _, err := pipe.ProcessStream(mic, out); err != nil {
		t.Fatal(err)
	}

	erle := metrics.MaxConvergedERLE(mic, out, 160)
	if bounds.MinERLE != nil && erle < *bounds.MinERLE {
		t.Errorf("ERLE = %.1f dB, want ≥ %.1f", erle, *bounds.MinERLE)
	}
}

// --- AEC: double-talk -----------------------------------------------------

func TestAEC_DoubleTalk(t *testing.T) {
	property.RequireImplemented(t, property.ModuleAEC)
	bounds := property.MustBounds(t, "aec_doubletalk")

	const rate = 16000
	const dur = rate * 3
	rng := rand.New(rand.NewPCG(2, 2))
	far := synth.SpeechShapedNoise(dur, rate, rng)
	ir := synth.ExpDecayIR(160, rate, 0.15, rng)
	echo := synth.Convolve(far, ir)[:dur]

	near := make([]float32, dur)
	mid0, mid1 := dur/4, 3*dur/4
	for i := mid0; i < mid1; i++ {
		near[i] = float32(rng.NormFloat64()) * 0.1 // synthetic near-end speech
	}
	mic := make([]float32, dur)
	for i := range mic {
		mic[i] = echo[i] + near[i]
	}

	cfg := apm.DefaultConfig()
	cfg.AEC.Enabled = true
	p, _ := apm.New(cfg)
	pipe := property.NewPipeline(p, apm.Rate16k)
	if err := pipe.ProcessReverseStream(far); err != nil {
		t.Fatal(err)
	}
	out := make([]float32, dur)
	if _, err := pipe.ProcessStream(mic, out); err != nil {
		t.Fatal(err)
	}

	// During single-talk regions only:
	erle := metrics.ERLE(mic[:mid0], out[:mid0], 160, -50)
	if bounds.MinERLE != nil && erle < *bounds.MinERLE {
		t.Errorf("single-talk ERLE = %.1f dB, want ≥ %.1f", erle, *bounds.MinERLE)
	}
}

// --- NS: SNR gain --------------------------------------------------------

func TestNS_SNRGain(t *testing.T) {
	property.RequireImplemented(t, property.ModuleNS)
	bounds := property.MustBounds(t, "ns_snr_gain")

	const rate = 16000
	const dur = rate * 2
	rng := rand.New(rand.NewPCG(3, 3))
	clean := synth.Sine(dur, rate, 250, 0.4)
	noise := synth.SpeechShapedNoise(dur, rate, rng)
	noisy, err := synth.MixAtSNR(clean, noise, 5.0)
	if err != nil {
		t.Fatal(err)
	}

	cfg := apm.DefaultConfig()
	cfg.NS = apm.NSConfig{Enabled: true, Level: apm.NSHigh}
	p, _ := apm.New(cfg)
	pipe := property.NewPipeline(p, apm.Rate16k)
	out := make([]float32, dur)
	if _, err := pipe.ProcessStream(noisy, out); err != nil {
		t.Fatal(err)
	}

	inSNR := metrics.SNR(clean, noisy)
	outSNR := metrics.SNR(clean, out)
	gain := outSNR - inSNR
	if bounds.MinSNRGain != nil && gain < *bounds.MinSNRGain {
		t.Errorf("NS SNR gain = %.1f dB, want ≥ %.1f", gain, *bounds.MinSNRGain)
	}
}

// --- NS: minimal speech distortion --------------------------------------

func TestNS_SpeechDistortion(t *testing.T) {
	property.RequireImplemented(t, property.ModuleNS)
	bounds := property.MustBounds(t, "ns_speech_distortion")

	const rate = 16000
	const dur = rate * 2
	rng := rand.New(rand.NewPCG(4, 4))
	clean := synth.Sweep(dur, rate, 100, 4000, 0.3)
	// add a tiny noise floor so NS engages
	noise := synth.WhiteNoise(dur, rng)
	for i := range clean {
		clean[i] += noise[i] * 0.005
	}

	cfg := apm.DefaultConfig()
	cfg.NS = apm.NSConfig{Enabled: true, Level: apm.NSModerate}
	p, _ := apm.New(cfg)
	pipe := property.NewPipeline(p, apm.Rate16k)
	out := make([]float32, dur)
	if _, err := pipe.ProcessStream(clean, out); err != nil {
		t.Fatal(err)
	}

	lsd := metrics.LogSpectralDistance(clean, out, rate, 512, 256)
	cd := metrics.CepstralDistance(clean, out, rate, 512, 256, 13)
	if bounds.MaxLSD != nil && lsd > *bounds.MaxLSD {
		t.Errorf("LSD = %.2f dB, want ≤ %.2f", lsd, *bounds.MaxLSD)
	}
	if bounds.MaxCepstralCD != nil && cd > *bounds.MaxCepstralCD {
		t.Errorf("CD = %.2f dB, want ≤ %.2f", cd, *bounds.MaxCepstralCD)
	}
}

// --- AGC: target level convergence ---------------------------------------

func TestAGC_TargetLevel(t *testing.T) {
	property.RequireImplemented(t, property.ModuleAGC)
	bounds := property.MustBounds(t, "agc_target_level")

	const rate = 16000
	const dur = rate * 2
	step := rate // step at 1 s
	in := make([]float32, dur)
	// Quiet first half (-30 dBFS), louder second half (-10 dBFS).
	quiet := synth.Sine(step, rate, 500, 0.0316) // -30 dBFS peak
	loud := synth.Sine(dur-step, rate, 500, 0.316)
	copy(in, quiet)
	copy(in[step:], loud)

	cfg := apm.DefaultConfig()
	cfg.AGC = apm.AGCConfig{Enabled: true, TargetLevelDBFS: -10, CompressionGain: 9}
	p, _ := apm.New(cfg)
	pipe := property.NewPipeline(p, apm.Rate16k)
	out := make([]float32, dur)
	if _, err := pipe.ProcessStream(in, out); err != nil {
		t.Fatal(err)
	}

	at := metrics.AttackTime(out, rate, step, 160, 3, 1.0)
	if bounds.MaxAttackS != nil && at > *bounds.MaxAttackS {
		t.Errorf("AGC attack = %.3f s, want ≤ %.3f", at, *bounds.MaxAttackS)
	}
}

// --- AGC: no peak overshoot ----------------------------------------------

func TestAGC_NoOvershoot(t *testing.T) {
	property.RequireImplemented(t, property.ModuleAGC)
	bounds := property.MustBounds(t, "agc_no_overshoot")

	const rate = 16000
	const dur = rate * 2
	rng := rand.New(rand.NewPCG(5, 5))
	in := synth.WhiteNoise(dur, rng)
	synth.ScaleToDBFS(in, 0) // drive AGC hard

	cfg := apm.DefaultConfig()
	cfg.AGC = apm.AGCConfig{Enabled: true, TargetLevelDBFS: -3, CompressionGain: 12}
	p, _ := apm.New(cfg)
	pipe := property.NewPipeline(p, apm.Rate16k)
	out := make([]float32, dur)
	if _, err := pipe.ProcessStream(in, out); err != nil {
		t.Fatal(err)
	}

	peak := metrics.PeakLevelDBFS(out)
	if bounds.MaxPeakDBFS != nil && peak > *bounds.MaxPeakDBFS {
		t.Errorf("AGC output peak = %.2f dBFS, want ≤ %.2f", peak, *bounds.MaxPeakDBFS)
	}
}

// --- HPF: corner attenuation --------------------------------------------

func TestHPF_Corner(t *testing.T) {
	property.RequireImplemented(t, property.ModuleHPF)
	bounds := property.MustBounds(t, "hpf_corner")

	const rate = 16000
	const dur = rate
	tone := synth.Sine(dur, rate, 80, 0.5) // at corner

	cfg := apm.DefaultConfig()
	cfg.HPF = apm.HPFConfig{Enabled: true}
	p, _ := apm.New(cfg)
	pipe := property.NewPipeline(p, apm.Rate16k)
	out := make([]float32, dur)
	if _, err := pipe.ProcessStream(tone, out); err != nil {
		t.Fatal(err)
	}

	inLvl := synth.LevelDBFS(tone)
	outLvl := synth.LevelDBFS(out)
	atten := outLvl - inLvl // negative
	if bounds.MinCornerDB != nil && atten > *bounds.MinCornerDB {
		t.Errorf("HPF @ corner attenuation = %.2f dB, want ≤ %.2f", atten, *bounds.MinCornerDB)
	}
}

// --- HPF: stopband DC rejection ------------------------------------------

func TestHPF_Stopband(t *testing.T) {
	property.RequireImplemented(t, property.ModuleHPF)
	bounds := property.MustBounds(t, "hpf_stopband")

	const rate = 16000
	const dur = rate
	tone := synth.Sine(dur, rate, 10, 0.5) // well below corner

	cfg := apm.DefaultConfig()
	cfg.HPF = apm.HPFConfig{Enabled: true}
	p, _ := apm.New(cfg)
	pipe := property.NewPipeline(p, apm.Rate16k)
	out := make([]float32, dur)
	if _, err := pipe.ProcessStream(tone, out); err != nil {
		t.Fatal(err)
	}

	inLvl := synth.LevelDBFS(tone)
	outLvl := synth.LevelDBFS(out)
	atten := outLvl - inLvl
	if bounds.MinCornerDB != nil && atten > *bounds.MinCornerDB {
		t.Errorf("HPF stopband attenuation = %.2f dB, want ≤ %.2f", atten, *bounds.MinCornerDB)
	}
}

// --- Harness smoke test: runs with all modules disabled ------------------
// Verifies the framework itself works end-to-end (synth → pipeline → metric)
// without relying on any module being implemented. Always runs.

func TestHarness_PassthroughSmoke(t *testing.T) {
	const rate = 16000
	const dur = rate
	rng := rand.New(rand.NewPCG(99, 99))
	in := synth.WhiteNoise(dur, rng)

	p, _ := apm.New(apm.DefaultConfig())
	pipe := property.NewPipeline(p, apm.Rate16k)
	out := make([]float32, dur)
	n, err := pipe.ProcessStream(in, out)
	if err != nil {
		t.Fatal(err)
	}
	if n != dur {
		t.Fatalf("processed %d samples, want %d", n, dur)
	}
	// Passthrough → input ≈ output, so SNR should be very high.
	snr := metrics.SNR(in, out)
	if snr < 60 {
		t.Errorf("passthrough SNR = %.1f dB, want very high", snr)
	}
}
