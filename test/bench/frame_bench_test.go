package bench

import (
	"testing"

	"github.com/fallais/gopam/apm"
)

// BenchmarkProcessStream_PassthroughMono16k measures the time and allocs
// to process one 10 ms mono frame at 16 kHz through the Processor.
// Target: <100 ns/op and 0 allocs/op for the eventual implemented modules.
func BenchmarkProcessStream_PassthroughMono16k(b *testing.B) {
	p, _ := apm.New(apm.DefaultConfig(apm.Rate16k, 1))
	frame := apm.NewFrame(apm.Rate16k, 1)
	// Warm up.
	if err := p.ProcessStream(frame); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.ProcessStream(frame)
	}
}

func BenchmarkProcessStream_PassthroughStereo48k(b *testing.B) {
	p, _ := apm.New(apm.DefaultConfig(apm.Rate48k, 2))
	frame := apm.NewFrame(apm.Rate48k, 2)
	_ = p.ProcessStream(frame)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.ProcessStream(frame)
	}
}

func BenchmarkProcessReverseStream_Mono16k(b *testing.B) {
	p, _ := apm.New(apm.DefaultConfig(apm.Rate16k, 1))
	frame := apm.NewFrame(apm.Rate16k, 1)
	_ = p.ProcessReverseStream(frame)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.ProcessReverseStream(frame)
	}
}

// BenchmarkFullPipeline_PassthroughMono16k drives both reverse and near
// streams as a realistic workload would, end-to-end per frame.
func BenchmarkFullPipeline_PassthroughMono16k(b *testing.B) {
	p, _ := apm.New(apm.DefaultConfig(apm.Rate16k, 1))
	near := apm.NewFrame(apm.Rate16k, 1)
	far := apm.NewFrame(apm.Rate16k, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.ProcessReverseStream(far)
		_ = p.ProcessStream(near)
	}
}
