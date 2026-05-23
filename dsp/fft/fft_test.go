package fft

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	for _, n := range []int{16, 64, 256, 1024} {
		p := New(n)
		rng := rand.New(rand.NewPCG(uint64(n), 7))
		in := make([]float32, n)
		for i := range in {
			in[i] = float32(rng.NormFloat64())
		}
		re := make([]float32, n/2+1)
		im := make([]float32, n/2+1)
		out := make([]float32, n)
		p.Forward(in, re, im)
		p.Inverse(re, im, out)
		for i, v := range in {
			if math.Abs(float64(v-out[i])) > 1e-4 {
				t.Fatalf("n=%d: i=%d in=%f out=%f", n, i, v, out[i])
			}
		}
	}
}

func TestForward_KnownTone(t *testing.T) {
	n := 256
	p := New(n)
	in := make([]float32, n)
	const bin = 5
	for i := range in {
		in[i] = float32(math.Cos(2 * math.Pi * float64(bin*i) / float64(n)))
	}
	re := make([]float32, n/2+1)
	im := make([]float32, n/2+1)
	p.Forward(in, re, im)
	// Energy should be concentrated at bin `bin`.
	var peakBin int
	peak := float32(-1)
	for k := range re {
		mag := re[k]*re[k] + im[k]*im[k]
		if mag > peak {
			peak = mag
			peakBin = k
		}
	}
	if peakBin != bin {
		t.Fatalf("peak bin = %d, want %d", peakBin, bin)
	}
}

func TestForward_ZeroAlloc(t *testing.T) {
	p := New(256)
	in := make([]float32, 256)
	re := make([]float32, 129)
	im := make([]float32, 129)
	if got := testing.AllocsPerRun(100, func() { p.Forward(in, re, im) }); got != 0 {
		t.Errorf("Forward allocates %.0f times/run", got)
	}
	if got := testing.AllocsPerRun(100, func() { p.Inverse(re, im, in) }); got != 0 {
		t.Errorf("Inverse allocates %.0f times/run", got)
	}
}

func BenchmarkForward256(b *testing.B) {
	p := New(256)
	in := make([]float32, 256)
	re := make([]float32, 129)
	im := make([]float32, 129)
	for i := range in {
		in[i] = float32(math.Sin(float64(i) * 0.1))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Forward(in, re, im)
	}
}
