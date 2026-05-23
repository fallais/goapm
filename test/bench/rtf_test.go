package bench

import (
	"testing"
	"time"

	"github.com/fallais/gopam/apm"
)

// TestRTF_PassthroughIsRealTime measures the real-time factor of a
// passthrough Processor over a fixed amount of audio. We assert RTF < 1
// (it must process audio faster than wall-clock) plus a generous margin.
//
// Once modules are implemented, this test stays valid — RTF will grow
// but the boundary is the same. Adjust the bound in the assertion if a
// real implementation legitimately approaches it.
func TestRTF_PassthroughIsRealTime(t *testing.T) {
	const (
		rate    = apm.Rate16k
		seconds = 10
	)
	p, _ := apm.New(apm.DefaultConfig(rate, 1))
	frame := apm.NewFrame(rate, 1)
	per := rate.SamplesPerFrame()
	framesPerSecond := int(rate) / per
	totalFrames := framesPerSecond * seconds

	start := time.Now()
	for i := 0; i < totalFrames; i++ {
		if err := p.ProcessStream(frame); err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)
	audioMS := float64(seconds * 1000)
	wallMS := float64(elapsed.Milliseconds())
	rtf := wallMS / audioMS

	t.Logf("RTF = %.4f (processed %d ms of audio in %d ms wall)", rtf, int(audioMS), int(wallMS))
	if rtf >= 0.1 {
		t.Errorf("passthrough RTF = %.4f, expected ≪ 0.1", rtf)
	}
}
