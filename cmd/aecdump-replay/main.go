// Command aecdump-replay reads a WebRTC .aecdump debug recording, drives
// the gopam APM Processor with its events, and writes a WAV of the
// near-end output. It is the gopam equivalent of upstream's audioproc_f.
//
// Usage:
//
//	aecdump-replay -in capture.aecdump -out result.wav
//	aecdump-replay -in capture.aecdump -out result.wav -force-ns -force-ns-level=2
//
// The -force-* flags override the configuration recorded in the dump,
// which is useful when validating one module at a time.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/fallais/gopam/apm"
	"github.com/fallais/gopam/audio/wav"
	webrtcproto "github.com/fallais/gopam/third_party/webrtc-proto"
)

func main() {
	in := flag.String("in", "", "input .aecdump file")
	out := flag.String("out", "", "output WAV file (near-end processed)")
	forceHPF := flag.Bool("force-hpf", false, "force HPF on regardless of dump config")
	forceNS := flag.Bool("force-ns", false, "force NS on")
	forceNSLevel := flag.Int("force-ns-level", -1, "override NS level (0..3); -1 leaves untouched")
	forceAGC := flag.Bool("force-agc", false, "force AGC on")
	forceAEC := flag.Bool("force-aec", false, "force AEC on")
	flag.Parse()

	if *in == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*in, *out, *forceHPF, *forceNS, *forceNSLevel, *forceAGC, *forceAEC); err != nil {
		log.Fatalf("aecdump-replay: %v", err)
	}
}

func run(inPath, outPath string, fHPF, fNS bool, fNSLevel int, fAGC, fAEC bool) error {
	f, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer f.Close()
	r := webrtcproto.NewReader(f)

	var (
		processor *apm.Processor
		pipeline  *replayPipeline
		writer    *wav.Writer
	)
	defer func() {
		if writer != nil {
			_ = writer.Close()
		}
	}()

	frameCount := 0
	for {
		ev, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("dump read at frame %d: %w", frameCount, err)
		}
		switch ev.Type {
		case webrtcproto.EventInit:
			if ev.Init == nil {
				return errors.New("INIT event without payload")
			}
			if processor != nil {
				return errors.New("multiple INIT events not supported")
			}
			processor, pipeline, err = setupFromInit(ev.Init, fHPF, fNS, fNSLevel, fAGC, fAEC)
			if err != nil {
				return err
			}
			writer, err = wav.CreateWriter(outPath, int(ev.Init.SampleRate), int(ev.Init.NumOutputChannels), wav.Float32)
			if err != nil {
				return err
			}
		case webrtcproto.EventConfig:
			// Surface the recorded config on stderr for transparency.
			if ev.Config != nil {
				log.Printf("dump config: aec=%t agc=%t ns=%t hpf=%t ns_lvl=%d",
					ev.Config.AECEnabled, ev.Config.AGCEnabled,
					ev.Config.NSEnabled, ev.Config.HPFEnabled, ev.Config.NSLevel)
			}
		case webrtcproto.EventReverseStream:
			if pipeline == nil {
				return errors.New("REVERSE_STREAM before INIT")
			}
			if ev.ReverseStream == nil || len(ev.ReverseStream.Channels) == 0 {
				continue // legacy interleaved path not supported in stub
			}
			if err := pipeline.processReverse(ev.ReverseStream.Channels); err != nil {
				return err
			}
		case webrtcproto.EventStream:
			if pipeline == nil {
				return errors.New("STREAM before INIT")
			}
			if ev.Stream == nil || len(ev.Stream.InputChannels) == 0 {
				continue
			}
			if ev.Stream.DelayMS > 0 {
				_ = processor.SetStreamDelayMS(int(ev.Stream.DelayMS))
			}
			out, err := pipeline.processStream(ev.Stream.InputChannels)
			if err != nil {
				return err
			}
			if err := writeInterleaved(writer, out); err != nil {
				return err
			}
			frameCount++
		}
	}
	log.Printf("replayed %d capture frames → %s", frameCount, outPath)
	return nil
}

type replayPipeline struct {
	p          *apm.Processor
	nearRate   apm.SampleRate
	farRate    apm.SampleRate
	nearFrame  *apm.Frame
	farFrame   *apm.Frame
}

func setupFromInit(init *webrtcproto.Init, fHPF, fNS bool, fNSLevel int, fAGC, fAEC bool) (*apm.Processor, *replayPipeline, error) {
	nearRate := apm.SampleRate(init.SampleRate)
	if !nearRate.Valid() {
		return nil, nil, fmt.Errorf("unsupported sample rate %d", init.SampleRate)
	}
	farRate := nearRate
	if init.ReverseSampleRate > 0 {
		farRate = apm.SampleRate(init.ReverseSampleRate)
	}
	cfg := apm.DefaultConfig()
	cfg.HPF.Enabled = fHPF
	cfg.NS.Enabled = fNS
	if fNSLevel >= 0 {
		cfg.NS.Level = apm.NSLevel(fNSLevel)
	}
	cfg.AGC.Enabled = fAGC
	cfg.AEC.Enabled = fAEC
	p, err := apm.New(cfg)
	if err != nil {
		return nil, nil, err
	}
	pipe := &replayPipeline{
		p:         p,
		nearRate:  nearRate,
		farRate:   farRate,
		nearFrame: apm.NewFrame(nearRate, int(init.NumInputChannels)),
		farFrame:  apm.NewFrame(farRate, int(init.NumReverseChannels)),
	}
	return p, pipe, nil
}

func (p *replayPipeline) processStream(in [][]float32) ([][]float32, error) {
	if len(in) != p.nearFrame.NumChannels() {
		return nil, fmt.Errorf("near frame channel mismatch: got %d want %d", len(in), p.nearFrame.NumChannels())
	}
	n := p.nearFrame.NumSamples()
	for c := range in {
		if len(in[c]) != n {
			return nil, fmt.Errorf("near frame[%d] len %d, want %d", c, len(in[c]), n)
		}
		copy(p.nearFrame.Data[c], in[c])
	}
	if err := p.p.ProcessStream(p.nearFrame); err != nil {
		return nil, err
	}
	out := make([][]float32, p.nearFrame.NumChannels())
	for c := range out {
		out[c] = append([]float32(nil), p.nearFrame.Data[c]...)
	}
	return out, nil
}

func (p *replayPipeline) processReverse(in [][]float32) error {
	if len(in) != p.farFrame.NumChannels() {
		return fmt.Errorf("far frame channel mismatch: got %d want %d", len(in), p.farFrame.NumChannels())
	}
	n := p.farFrame.NumSamples()
	for c := range in {
		if len(in[c]) != n {
			return fmt.Errorf("far frame[%d] len %d, want %d", c, len(in[c]), n)
		}
		copy(p.farFrame.Data[c], in[c])
	}
	return p.p.ProcessReverseStream(p.farFrame)
}

func writeInterleaved(w *wav.Writer, channels [][]float32) error {
	if len(channels) == 0 {
		return nil
	}
	n := len(channels[0])
	buf := make([]float32, n*len(channels))
	for i := 0; i < n; i++ {
		for c := range channels {
			buf[i*len(channels)+c] = channels[c][i]
		}
	}
	return w.WriteFloat32(buf)
}
