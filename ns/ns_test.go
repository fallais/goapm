package ns

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestNewSuppressor_RejectsBadRate(t *testing.T) {
	if _, err := NewSuppressor(DefaultConfig(), 8000, 1); err == nil {
		t.Error("expected error at 8 kHz")
	}
	if _, err := NewSuppressor(DefaultConfig(), 48000, 1); err == nil {
		t.Error("expected error at 48 kHz (multi-band not yet supported)")
	}
}

func TestSuppressor_FrameLength(t *testing.T) {
	s, err := NewSuppressor(DefaultConfig(), 16000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Process([][]float32{make([]float32, 100)}); err == nil {
		t.Error("expected length error")
	}
	if err := s.Analyze([][]float32{make([]float32, 100)}); err == nil {
		t.Error("expected length error")
	}
}

func TestSuppressor_NoiseReduction(t *testing.T) {
	s, err := NewSuppressor(Config{TargetLevel: Level18dB}, 16000, 1)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewPCG(1, 2))

	const frames = 400
	noise := make([]float32, frames*nsFrameSize)
	for i := range noise {
		noise[i] = float32(rng.NormFloat64()) * 100
	}

	inPower := powerOf(noise)

	// Train and process — give NS time to learn the noise model.
	out := make([]float32, frames*nsFrameSize)
	for f := 0; f < frames; f++ {
		buf := []float32{noise[f*nsFrameSize : (f+1)*nsFrameSize][0]}
		_ = buf // silence warning; we'll use slices below
	}

	for f := 0; f < frames; f++ {
		frame := append([]float32(nil), noise[f*nsFrameSize:(f+1)*nsFrameSize]...)
		if err := s.Analyze([][]float32{frame}); err != nil {
			t.Fatal(err)
		}
		if err := s.Process([][]float32{frame}); err != nil {
			t.Fatal(err)
		}
		copy(out[f*nsFrameSize:], frame)
	}
	// Examine steady-state output (skip first 200 frames of adaptation).
	steady := out[200*nsFrameSize:]
	outPower := powerOf(steady)
	if outPower <= 0 || inPower <= 0 {
		t.Fatalf("zero power: in=%f out=%f", inPower, outPower)
	}
	atten := 10 * math.Log10(outPower/inPower)
	if atten > -3 {
		t.Errorf("expected noise to be attenuated; got %.1f dB change (want < -3 dB)", atten)
	}
	t.Logf("NS attenuation on pure noise (steady): %.1f dB", atten)
}

func TestSuppressor_PreservesAmplitudeModulatedSpeech(t *testing.T) {
	// A stationary tone looks like noise to NS by design. Speech-like
	// signals have an envelope; this test uses an AM-modulated multi-tone
	// (closer to voiced speech) and asserts NS doesn't gut it.
	s, err := NewSuppressor(Config{TargetLevel: Level12dB}, 16000, 1)
	if err != nil {
		t.Fatal(err)
	}
	const frames = 400
	const rate = 16000
	signal := make([]float32, frames*nsFrameSize)
	for i := range signal {
		t := float64(i) / float64(rate)
		// Amplitude-modulated speech-like signal: three harmonics at 200/400/600 Hz
		// modulated by a 5 Hz envelope.
		env := 0.5 + 0.5*math.Sin(2*math.Pi*5*t)
		carrier := math.Sin(2*math.Pi*200*t) + 0.7*math.Sin(2*math.Pi*400*t) + 0.4*math.Sin(2*math.Pi*600*t)
		signal[i] = float32(env * carrier * 300)
	}

	out := make([]float32, frames*nsFrameSize)
	for f := 0; f < frames; f++ {
		frame := append([]float32(nil), signal[f*nsFrameSize:(f+1)*nsFrameSize]...)
		if err := s.Analyze([][]float32{frame}); err != nil {
			t.Fatal(err)
		}
		if err := s.Process([][]float32{frame}); err != nil {
			t.Fatal(err)
		}
		copy(out[f*nsFrameSize:], frame)
	}
	steady := out[200*nsFrameSize:]
	steadyIn := signal[200*nsFrameSize:]
	atten := 10 * math.Log10(powerOf(steady) / powerOf(steadyIn))
	t.Logf("AM-multi-tone attenuation (steady): %.1f dB", atten)
	if atten < -10 {
		t.Errorf("speech-like signal over-attenuated: %.1f dB", atten)
	}
}

func powerOf(x []float32) float64 {
	var s float64
	for _, v := range x {
		s += float64(v) * float64(v)
	}
	return s / float64(len(x))
}
