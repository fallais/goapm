package hpf

import (
	"math"
	"testing"
)

func TestNew_UnsupportedRate(t *testing.T) {
	if _, err := New(8000, 1); err == nil {
		t.Error("expected error for 8 kHz")
	}
	if _, err := New(44100, 1); err == nil {
		t.Error("expected error for 44.1 kHz")
	}
}

func TestProcess_DCRejection(t *testing.T) {
	for _, rate := range []int{16000, 32000, 48000} {
		f, err := New(rate, 1)
		if err != nil {
			t.Fatal(err)
		}
		n := rate / 4 // 250 ms — well past convergence
		in := make([]float32, n)
		for i := range in {
			in[i] = 1.0
		}
		if err := f.Process([][]float32{in}); err != nil {
			t.Fatal(err)
		}
		// Examine the converged tail.
		var sum float64
		tail := in[n/2:]
		for _, v := range tail {
			sum += math.Abs(float64(v))
		}
		avg := sum / float64(len(tail))
		if avg > 1e-3 {
			t.Errorf("%d Hz: DC residual avg = %f", rate, avg)
		}
	}
}

func TestProcess_HighFreqPassband(t *testing.T) {
	for _, rate := range []int{16000, 32000, 48000} {
		f, err := New(rate, 1)
		if err != nil {
			t.Fatal(err)
		}
		n := rate / 2
		freq := 1000.0
		in := make([]float32, n)
		ref := make([]float32, n)
		for i := range in {
			in[i] = float32(math.Sin(2 * math.Pi * freq * float64(i) / float64(rate)))
			ref[i] = in[i]
		}
		if err := f.Process([][]float32{in}); err != nil {
			t.Fatal(err)
		}
		var sIn, sOut float64
		for i := 1000; i < n; i++ {
			sIn += float64(ref[i]) * float64(ref[i])
			sOut += float64(in[i]) * float64(in[i])
		}
		ratio := sOut / sIn
		if ratio < 0.9 || ratio > 1.1 {
			t.Errorf("%d Hz: passband ratio = %f", rate, ratio)
		}
	}
}

func TestProcess_LowFreqRejection(t *testing.T) {
	// 10 Hz tone should be attenuated by far more than 20 dB.
	rate := 16000
	f, _ := New(rate, 1)
	n := rate
	freq := 10.0
	in := make([]float32, n)
	ref := make([]float32, n)
	for i := range in {
		in[i] = float32(math.Sin(2 * math.Pi * freq * float64(i) / float64(rate)))
		ref[i] = in[i]
	}
	if err := f.Process([][]float32{in}); err != nil {
		t.Fatal(err)
	}
	var sIn, sOut float64
	for i := 2000; i < n; i++ {
		sIn += float64(ref[i]) * float64(ref[i])
		sOut += float64(in[i]) * float64(in[i])
	}
	if sOut == 0 {
		return
	}
	attenDB := 10 * math.Log10(sOut/sIn)
	if attenDB > -20 {
		t.Errorf("10 Hz attenuation = %.1f dB, want ≤ -20", attenDB)
	}
}

func TestProcess_MultiChannelIndependent(t *testing.T) {
	f, _ := New(16000, 2)
	left := make([]float32, 160)
	right := make([]float32, 160)
	for i := range left {
		left[i] = 1.0 // DC on left
		right[i] = float32(math.Sin(2 * math.Pi * 1000 * float64(i) / 16000))
	}
	rightRef := append([]float32(nil), right...)
	if err := f.Process([][]float32{left, right}); err != nil {
		t.Fatal(err)
	}
	// Right channel should be largely preserved (1 kHz passband).
	var diff float64
	for i := range right {
		d := float64(right[i] - rightRef[i])
		diff += d * d
	}
	rmse := math.Sqrt(diff / float64(len(right)))
	if rmse > 0.3 {
		t.Errorf("right-channel distortion rmse = %f, want small", rmse)
	}
}

func TestProcess_ChannelCountMismatch(t *testing.T) {
	f, _ := New(16000, 2)
	if err := f.Process([][]float32{make([]float32, 160)}); err == nil {
		t.Error("expected channel mismatch error")
	}
}

func TestProcess_ZeroAlloc(t *testing.T) {
	f, _ := New(16000, 1)
	in := [][]float32{make([]float32, 160)}
	_ = f.Process(in)
	if got := testing.AllocsPerRun(100, func() { _ = f.Process(in) }); got != 0 {
		t.Errorf("Process allocates %.0f/run", got)
	}
}

func TestReset_ClearsState(t *testing.T) {
	f, _ := New(16000, 1)
	in := make([]float32, 160)
	for i := range in {
		in[i] = 1.0
	}
	_ = f.Process([][]float32{in})
	f.Reset()
	// Re-process DC; if reset worked, output starts large again
	// (transient before convergence) instead of being already-converged.
	in2 := make([]float32, 160)
	for i := range in2 {
		in2[i] = 1.0
	}
	_ = f.Process([][]float32{in2})
	if math.Abs(float64(in2[0])) < 0.1 {
		t.Errorf("after Reset(), output[0] = %f — expected large transient", in2[0])
	}
}
