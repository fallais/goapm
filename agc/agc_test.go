package agc

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestNew_RejectsBadShape(t *testing.T) {
	if _, err := New(Config{}, 16000, 0); err == nil {
		t.Error("expected error for 0 channels")
	}
	if _, err := New(Config{}, 11025, 1); err == nil {
		t.Error("expected error for non-divisible sample rate")
	}
}

func TestController_LimiterPreventsOvershoot(t *testing.T) {
	c, err := New(Config{FixedDigitalGainDB: 12}, 16000, 1)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewPCG(1, 1))
	const frames = 200
	for f := 0; f < frames; f++ {
		ch := make([]float32, 160)
		for i := range ch {
			ch[i] = float32(rng.NormFloat64() * 20000) // already loud
		}
		if err := c.Process([][]float32{ch}); err != nil {
			t.Fatal(err)
		}
		var peak float32
		for _, v := range ch {
			a := v
			if a < 0 {
				a = -a
			}
			if a > peak {
				peak = a
			}
		}
		if peak > kMaxFloatS16Value+1 {
			t.Errorf("frame %d peak %f exceeded ceiling", f, peak)
		}
	}
}

func TestController_QuietSignalRaisedByFixedGain(t *testing.T) {
	c, err := New(Config{FixedDigitalGainDB: 12}, 16000, 1)
	if err != nil {
		t.Fatal(err)
	}
	const frames = 100
	var rmsIn, rmsOut float64
	for f := 0; f < frames; f++ {
		in := make([]float32, 160)
		for i := range in {
			in[i] = float32(math.Sin(2*math.Pi*1000*float64(f*160+i)/16000)) * 500 // quiet
		}
		ref := append([]float32(nil), in...)
		if err := c.Process([][]float32{in}); err != nil {
			t.Fatal(err)
		}
		// Skip warmup ramp.
		if f < 10 {
			continue
		}
		for i := range in {
			rmsIn += float64(ref[i]) * float64(ref[i])
			rmsOut += float64(in[i]) * float64(in[i])
		}
	}
	if rmsOut <= rmsIn {
		t.Errorf("fixed +12 dB gain should raise level; got rmsIn=%f rmsOut=%f", math.Sqrt(rmsIn), math.Sqrt(rmsOut))
	}
}

func TestController_ZeroAllocSteadyState(t *testing.T) {
	c, _ := New(Config{FixedDigitalGainDB: 6}, 16000, 1)
	ch := make([]float32, 160)
	for i := range ch {
		ch[i] = 1000
	}
	_ = c.Process([][]float32{ch})
	allocs := testing.AllocsPerRun(100, func() {
		_ = c.Process([][]float32{ch})
	})
	if allocs != 0 {
		t.Errorf("Process allocates %.0f/run", allocs)
	}
}
