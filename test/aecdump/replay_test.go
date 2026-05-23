package aecdump

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/fallais/gopam/apm"
	"github.com/fallais/gopam/audio/wav"
	webrtcproto "github.com/fallais/gopam/third_party/webrtc-proto"
)

// TestReplay_PassthroughRoundTrip writes a synthetic 1-second .aecdump,
// replays it through a passthrough Processor, and verifies the output
// WAV has the right shape and samples. This is the smoke test for the
// whole .aecdump → Processor → WAV path, independent of any module.
func TestReplay_PassthroughRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dumpPath := filepath.Join(dir, "synthetic.aecdump")
	outPath := filepath.Join(dir, "out.wav")

	const rate = 16000
	const frames = 100 // 1 second of 10 ms frames
	const samplesPerFrame = 160
	if err := writeSyntheticDump(dumpPath, rate, frames); err != nil {
		t.Fatal(err)
	}
	if err := runReplay(dumpPath, outPath); err != nil {
		t.Fatal(err)
	}
	channels, gotRate, err := wav.ReadAll(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if gotRate != rate {
		t.Fatalf("rate = %d, want %d", gotRate, rate)
	}
	if len(channels) != 1 {
		t.Fatalf("channels = %d, want 1", len(channels))
	}
	if got := len(channels[0]); got != frames*samplesPerFrame {
		t.Fatalf("output length = %d, want %d", got, frames*samplesPerFrame)
	}
	// Passthrough preserves samples; spot-check a few.
	for i := 0; i < 5; i++ {
		idx := i * samplesPerFrame
		// Float32 round-trips exactly through WAV.
		expect := float32(idx%2000) / 2000
		if channels[0][idx] != expect {
			t.Errorf("sample %d = %f, want %f", idx, channels[0][idx], expect)
		}
	}
}

func writeSyntheticDump(path string, rate, frames int) error {
	f, err := openFileForWrite(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := webrtcproto.WriteEvent(f, &webrtcproto.Event{
		Type: webrtcproto.EventInit,
		Init: &webrtcproto.Init{
			SampleRate:         int32(rate),
			NumInputChannels:   1,
			NumOutputChannels:  1,
			NumReverseChannels: 1,
			ReverseSampleRate:  int32(rate),
			OutputSampleRate:   int32(rate),
		},
	}); err != nil {
		return err
	}
	per := rate * 10 / 1000 // samples per 10 ms frame
	for i := 0; i < frames; i++ {
		near := make([]float32, per)
		far := make([]float32, per)
		for j := range near {
			idx := i*per + j
			v := float32(idx%2000) / 2000
			near[j] = v
			far[j] = v * 0.5
		}
		if err := webrtcproto.WriteEvent(f, &webrtcproto.Event{
			Type:          webrtcproto.EventReverseStream,
			ReverseStream: &webrtcproto.ReverseStream{Channels: [][]float32{far}},
		}); err != nil {
			return err
		}
		if err := webrtcproto.WriteEvent(f, &webrtcproto.Event{
			Type: webrtcproto.EventStream,
			Stream: &webrtcproto.Stream{
				DelayMS:       50,
				InputChannels: [][]float32{near},
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

// openFileForWrite is split out so the test doesn't need to import "os"
// at the test-file scope alongside our other helpers.
func openFileForWrite(path string) (io.WriteCloser, error) {
	return osCreate(path)
}

// runReplay is a thin in-process equivalent of `cmd/aecdump-replay`,
// duplicated here so tests don't have to exec the built binary.
func runReplay(dumpPath, outPath string) error {
	src, err := osOpen(dumpPath)
	if err != nil {
		return err
	}
	defer src.Close()
	r := webrtcproto.NewReader(src)

	var (
		processor *apm.Processor
		writer    *wav.Writer
		nearFrame *apm.Frame
		farFrame  *apm.Frame
	)
	defer func() {
		if writer != nil {
			_ = writer.Close()
		}
	}()
	for {
		ev, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch ev.Type {
		case webrtcproto.EventInit:
			rate := apm.SampleRate(ev.Init.SampleRate)
			processor, _ = apm.New(apm.DefaultConfig(rate, int(ev.Init.NumInputChannels)))
			nearFrame = apm.NewFrame(rate, int(ev.Init.NumInputChannels))
			farFrame = apm.NewFrame(apm.SampleRate(ev.Init.ReverseSampleRate), int(ev.Init.NumReverseChannels))
			writer, err = wav.CreateWriter(outPath, int(rate), int(ev.Init.NumInputChannels), wav.Float32)
			if err != nil {
				return err
			}
		case webrtcproto.EventReverseStream:
			for c, ch := range ev.ReverseStream.Channels {
				copy(farFrame.Data[c], ch)
			}
			_ = processor.ProcessReverseStream(farFrame)
		case webrtcproto.EventStream:
			for c, ch := range ev.Stream.InputChannels {
				copy(nearFrame.Data[c], ch)
			}
			_ = processor.ProcessStream(nearFrame)
			// interleave and write
			n := nearFrame.NumSamples()
			ch := nearFrame.NumChannels()
			buf := make([]float32, n*ch)
			for i := 0; i < n; i++ {
				for c := 0; c < ch; c++ {
					buf[i*ch+c] = nearFrame.Data[c][i]
				}
			}
			if err := writer.WriteFloat32(buf); err != nil {
				return err
			}
		}
	}
	return nil
}

// Tiny os-package shims, isolated for testability without polluting imports above.
var (
	osCreate = func(path string) (io.WriteCloser, error) { return osCreateImpl(path) }
	osOpen   = func(path string) (io.ReadCloser, error) { return osOpenImpl(path) }
)

func TestSilence_AvoidsUnusedVarLinter(_ *testing.T) {
	// Force-touch the io.Reader contract on a bytes.Buffer to ensure
	// imports stay live even when other tests don't exercise them.
	var b bytes.Buffer
	_, _ = b.Write(nil)
}
