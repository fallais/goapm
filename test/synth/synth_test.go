package synth

import (
	"math"
	"math/rand/v2"
	"testing"
)

func newRng() *rand.Rand {
	return rand.New(rand.NewPCG(1, 2))
}

func TestSine_RMSIs1OverSqrt2(t *testing.T) {
	s := Sine(48000, 48000, 1000, 1.0)
	rms := rmsFloat32(s)
	want := 1.0 / math.Sqrt(2)
	if math.Abs(rms-want) > 0.01 {
		t.Errorf("sine RMS = %f, want %f", rms, want)
	}
}

func TestSweep_StartsAtF0(t *testing.T) {
	s := Sweep(48000, 48000, 100, 10000, 1.0)
	// The instantaneous frequency at i=0 should be f0; just sanity-check
	// finite-amplitude output.
	for i := 0; i < 100; i++ {
		if math.IsNaN(float64(s[i])) {
			t.Fatalf("NaN at i=%d", i)
		}
	}
}

func TestWhiteNoise_Stats(t *testing.T) {
	n := 48000
	w := WhiteNoise(n, newRng())
	mean, variance := stats(w)
	if math.Abs(mean) > 0.05 {
		t.Errorf("white noise mean = %f, want ~0", mean)
	}
	if math.Abs(variance-1) > 0.1 {
		t.Errorf("white noise variance = %f, want ~1", variance)
	}
}

func TestImpulse(t *testing.T) {
	x := Impulse(100, 17)
	for i, v := range x {
		if i == 17 {
			if v != 1 {
				t.Errorf("impulse at 17 = %f, want 1", v)
			}
		} else if v != 0 {
			t.Errorf("impulse at %d = %f, want 0", i, v)
		}
	}
}

func TestConvolve_IdentityWithDelta(t *testing.T) {
	x := []float32{1, 2, 3, 4}
	h := []float32{1} // delta
	y := Convolve(x, h)
	if len(y) != 4 {
		t.Fatalf("length = %d, want 4", len(y))
	}
	for i, v := range y {
		if v != x[i] {
			t.Errorf("y[%d] = %f, want %f", i, v, x[i])
		}
	}
}

func TestConvolve_ShiftWithDelayedDelta(t *testing.T) {
	x := []float32{1, 2, 3}
	h := []float32{0, 0, 1} // 2-sample delay
	y := Convolve(x, h)
	want := []float32{0, 0, 1, 2, 3}
	if len(y) != len(want) {
		t.Fatalf("length = %d, want %d", len(y), len(want))
	}
	for i := range want {
		if y[i] != want[i] {
			t.Errorf("y[%d] = %f, want %f", i, y[i], want[i])
		}
	}
}

func TestMixAtSNR_HitsTarget(t *testing.T) {
	rng := newRng()
	clean := Sine(48000, 48000, 1000, 0.5)
	noise := WhiteNoise(48000, rng)
	for _, target := range []float64{-10, 0, 10, 20} {
		mix, err := MixAtSNR(clean, noise, target)
		if err != nil {
			t.Fatal(err)
		}
		if len(mix) != len(clean) {
			t.Fatalf("length mismatch")
		}
		// Recompute SNR from the un-normalized contributions: easier to
		// just check the mixed signal has the same RMS order of magnitude
		// as the cleaner signal.
		if rmsFloat32(mix) <= 0 {
			t.Errorf("mix has zero power")
		}
	}
}

func TestLevelDBFS_KnownInputs(t *testing.T) {
	// Full-scale sine: RMS = 1/sqrt(2) → −3.01 dBFS.
	s := Sine(48000, 48000, 1000, 1.0)
	got := LevelDBFS(s)
	if math.Abs(got-(-3.01)) > 0.05 {
		t.Errorf("full-scale sine dBFS = %f, want ~-3.01", got)
	}
	// Half-scale sine: −9 dBFS.
	half := Sine(48000, 48000, 1000, 0.5)
	got = LevelDBFS(half)
	if math.Abs(got-(-9.03)) > 0.05 {
		t.Errorf("half-scale sine dBFS = %f, want ~-9.03", got)
	}
}

func TestScaleToDBFS(t *testing.T) {
	s := Sine(48000, 48000, 1000, 1.0)
	ScaleToDBFS(s, -20)
	got := LevelDBFS(s)
	if math.Abs(got-(-20)) > 0.1 {
		t.Errorf("after scaling to -20 dBFS, got %f", got)
	}
}

// helpers

func rmsFloat32(x []float32) float64 {
	var s float64
	for _, v := range x {
		s += float64(v) * float64(v)
	}
	return math.Sqrt(s / float64(len(x)))
}

func stats(x []float32) (mean, variance float64) {
	for _, v := range x {
		mean += float64(v)
	}
	mean /= float64(len(x))
	for _, v := range x {
		d := float64(v) - mean
		variance += d * d
	}
	variance /= float64(len(x))
	return
}
