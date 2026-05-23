// Command rtf-bench measures the real-time factor (RTF) of a Processor
// configuration over a configurable amount of synthetic audio. It is the
// standalone equivalent of TestRTF_PassthroughIsRealTime, useful for
// profiling under pprof or comparing branches outside of `go test`.
//
// RTF = wall_clock_seconds / audio_seconds. RTF < 1 means real-time.
//
// Usage:
//
//	rtf-bench -seconds 30 -rate 48000 -channels 2 -ns -agc
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"runtime/pprof"
	"time"

	"github.com/fallais/gopam/apm"
	"github.com/fallais/gopam/test/synth"
)

func main() {
	seconds := flag.Int("seconds", 30, "audio duration to process (s)")
	rate := flag.Int("rate", 16000, "sample rate (Hz): 8000|16000|32000|48000")
	channels := flag.Int("channels", 1, "channel count")
	enableHPF := flag.Bool("hpf", false, "enable HPF")
	enableNS := flag.Bool("ns", false, "enable NS")
	enableAGC := flag.Bool("agc", false, "enable AGC")
	enableAEC := flag.Bool("aec", false, "enable AEC")
	cpuProfile := flag.String("cpuprofile", "", "write CPU profile to file")
	jsonOut := flag.String("json", "", "write a one-line JSON result to file")
	flag.Parse()

	sr := apm.SampleRate(*rate)
	if !sr.Valid() {
		log.Fatalf("invalid -rate %d", *rate)
	}
	cfg := apm.DefaultConfig(sr, *channels)
	cfg.HPF.Enabled = *enableHPF
	cfg.NS.Enabled = *enableNS
	cfg.AGC.Enabled = *enableAGC
	cfg.AEC.Enabled = *enableAEC
	p, err := apm.New(cfg)
	if err != nil {
		log.Fatalf("apm.New: %v", err)
	}

	per := sr.SamplesPerFrame()
	totalFrames := int(*rate) * (*seconds) / per
	rng := rand.New(rand.NewPCG(7, 7))
	frame := apm.NewFrame(sr, *channels)
	// Pre-fill with white noise so the workload isn't trivial-zero data.
	for c := 0; c < *channels; c++ {
		copy(frame.Data[c], synth.WhiteNoise(per, rng))
	}
	_ = p.ProcessStream(frame) // warm-up

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatalf("create cpu profile: %v", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatalf("start cpu profile: %v", err)
		}
		defer pprof.StopCPUProfile()
	}

	start := time.Now()
	for i := 0; i < totalFrames; i++ {
		_ = p.ProcessStream(frame)
	}
	wall := time.Since(start)
	audio := time.Duration(*seconds) * time.Second
	rtf := float64(wall) / float64(audio)

	result := map[string]any{
		"seconds":    *seconds,
		"rate":       *rate,
		"channels":   *channels,
		"frames":     totalFrames,
		"wall_ns":    wall.Nanoseconds(),
		"rtf":        rtf,
		"config":     cfg,
	}
	fmt.Printf("RTF=%.4f audio=%v wall=%v frames=%d cfg=%+v\n", rtf, audio, wall, totalFrames, cfg)
	if *jsonOut != "" {
		f, err := os.Create(*jsonOut)
		if err != nil {
			log.Fatalf("create json out: %v", err)
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		if err := enc.Encode(result); err != nil {
			log.Fatalf("encode json: %v", err)
		}
	}
}
