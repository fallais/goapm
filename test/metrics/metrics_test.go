package metrics

import (
	"math"
	"math/rand/v2"
	"testing"
)

// --- helpers --------------------------------------------------------------

func sine(n, rate int, f, amp float64) []float32 {
	out := make([]float32, n)
	w := 2 * math.Pi * f / float64(rate)
	for i := 0; i < n; i++ {
		out[i] = float32(amp * math.Sin(w*float64(i)))
	}
	return out
}

func white(n int, src *rand.Rand) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = float32(src.NormFloat64() * 0.1)
	}
	return out
}

// --- snr ------------------------------------------------------------------

func TestSNR_IdenticalIsInfinite(t *testing.T) {
	x := sine(16000, 16000, 1000, 0.5)
	if !math.IsInf(SNR(x, x), +1) {
		t.Errorf("identical → SNR should be +Inf")
	}
}

func TestSNR_HalvedReferenceMatchesExpected(t *testing.T) {
	x := sine(16000, 16000, 1000, 1.0)
	y := make([]float32, len(x))
	for i := range x {
		y[i] = x[i] * 0.5 // error = x - 0.5x = 0.5x → err power = 0.25·sig → SNR = 6.02 dB
	}
	snr := SNR(x, y)
	if math.Abs(snr-6.02) > 0.05 {
		t.Errorf("expected ~6 dB SNR for half-amplitude error, got %f", snr)
	}
}

func TestSegmentalSNR_Sensible(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	ref := sine(16000, 16000, 1000, 0.5)
	test := append([]float32(nil), ref...)
	noise := white(16000, rng)
	for i := range test {
		test[i] += noise[i] * 0.05
	}
	got := SegmentalSNR(ref, test, 160, -50)
	if math.IsNaN(got) || got <= 0 {
		t.Errorf("segmental SNR should be positive for a near-clean signal, got %v", got)
	}
}

// --- erle -----------------------------------------------------------------

func TestERLE_PerfectCancellation(t *testing.T) {
	mic := sine(16000, 16000, 1000, 0.5)
	residual := make([]float32, len(mic)) // all zeros
	erle := ERLE(mic, residual, 160, -50)
	if erle < 60 {
		t.Errorf("perfect cancellation should give large ERLE, got %f", erle)
	}
}

func TestERLE_NoCancellation(t *testing.T) {
	mic := sine(16000, 16000, 1000, 0.5)
	residual := append([]float32(nil), mic...) // identical → 0 dB
	erle := ERLE(mic, residual, 160, -50)
	if math.Abs(erle) > 0.5 {
		t.Errorf("no cancellation should give ~0 dB ERLE, got %f", erle)
	}
}

// --- lsd ------------------------------------------------------------------

func TestLSD_IdenticalIsZero(t *testing.T) {
	x := sine(8192, 16000, 1000, 0.5)
	lsd := LogSpectralDistance(x, x, 16000, 512, 256)
	if lsd > 0.01 {
		t.Errorf("identical signals → LSD ~0, got %f", lsd)
	}
}

func TestLSD_DifferentFreqIsPositive(t *testing.T) {
	x := sine(8192, 16000, 1000, 0.5)
	y := sine(8192, 16000, 2000, 0.5)
	lsd := LogSpectralDistance(x, y, 16000, 512, 256)
	if lsd <= 1 {
		t.Errorf("different tones should have meaningful LSD, got %f", lsd)
	}
}

// --- cepstral -------------------------------------------------------------

func TestCepstral_IdenticalIsZero(t *testing.T) {
	x := sine(8192, 16000, 1000, 0.5)
	cd := CepstralDistance(x, x, 16000, 512, 256, 13)
	if cd > 0.01 {
		t.Errorf("identical → CD ~0, got %f", cd)
	}
}

// --- attack ---------------------------------------------------------------

func TestAttackTime_StepResponds(t *testing.T) {
	rate := 16000
	dur := rate // 1 s
	step := dur / 2
	out := make([]float32, dur)
	for i := 0; i < step; i++ {
		out[i] = float32(math.Sin(2*math.Pi*1000*float64(i)/float64(rate)) * 0.05)
	}
	for i := step; i < dur; i++ {
		out[i] = float32(math.Sin(2*math.Pi*1000*float64(i)/float64(rate)) * 0.5)
	}
	at := AttackTime(out, rate, step, 80, 3, 1.0)
	if math.IsNaN(at) {
		t.Fatalf("attack should converge")
	}
	if at > 0.1 || at < 0 {
		t.Errorf("attack time = %f s, expected small positive", at)
	}
}

func TestPeakLevelDBFS(t *testing.T) {
	x := []float32{0.5, -0.5, 0.25}
	got := PeakLevelDBFS(x)
	want := 20 * math.Log10(0.5)
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("peak dBFS = %f, want %f", got, want)
	}
}

// --- thd ------------------------------------------------------------------

func TestTHDPlusN_CleanSineIsLow(t *testing.T) {
	x := sine(8192, 16000, 1000, 0.5)
	thd := THDPlusN(x, 16000, 1000)
	if thd > -40 {
		t.Errorf("clean sine THD+N should be very low, got %f dB", thd)
	}
}

func TestTHDPlusN_NoiseRaisesIt(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	x := sine(8192, 16000, 1000, 0.5)
	n := white(8192, rng)
	for i := range x {
		x[i] += n[i]
	}
	thd := THDPlusN(x, 16000, 1000)
	if thd < -30 {
		t.Errorf("noise injection should raise THD+N, got %f", thd)
	}
}

// --- peaq_lite ------------------------------------------------------------

func TestPEAQLite_Identical(t *testing.T) {
	x := sine(16384, 16000, 1000, 0.5)
	odg := PEAQLite(x, x, 16000)
	if odg < -0.1 || odg > 0 {
		t.Errorf("identical → ODG ~0, got %f", odg)
	}
}
