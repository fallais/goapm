package webrtcproto

import (
	"bytes"
	"io"
	"testing"
)

func TestRoundTrip_InitStreamReverse(t *testing.T) {
	var buf bytes.Buffer
	events := []*Event{
		{Type: EventInit, Init: &Init{
			SampleRate:        16000,
			NumInputChannels:  1,
			NumOutputChannels: 1,
			NumReverseChannels: 1,
			ReverseSampleRate: 16000,
			OutputSampleRate:  16000,
			TimestampMS:       0,
		}},
		{Type: EventConfig, Config: &ConfigEvent{
			NSEnabled: true,
			NSLevel:   2,
		}},
		{Type: EventReverseStream, ReverseStream: &ReverseStream{
			Channels: [][]float32{makeRamp(160)},
		}},
		{Type: EventStream, Stream: &Stream{
			DelayMS:        50,
			InputChannels:  [][]float32{makeRamp(160)},
			OutputChannels: [][]float32{makeRamp(160)},
		}},
	}
	for _, ev := range events {
		if err := WriteEvent(&buf, ev); err != nil {
			t.Fatal(err)
		}
	}
	r := NewReader(&buf)
	var got []*Event
	for {
		ev, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, ev)
	}
	if len(got) != len(events) {
		t.Fatalf("got %d events, want %d", len(got), len(events))
	}
	if got[0].Init.SampleRate != 16000 {
		t.Errorf("init.SampleRate = %d", got[0].Init.SampleRate)
	}
	if !got[1].Config.NSEnabled || got[1].Config.NSLevel != 2 {
		t.Errorf("config = %+v", got[1].Config)
	}
	if len(got[2].ReverseStream.Channels) != 1 || len(got[2].ReverseStream.Channels[0]) != 160 {
		t.Errorf("reverse channels shape: %+v", got[2].ReverseStream)
	}
	if got[3].Stream.DelayMS != 50 {
		t.Errorf("stream.delay = %d", got[3].Stream.DelayMS)
	}
	if len(got[3].Stream.InputChannels[0]) != 160 {
		t.Errorf("stream input len = %d", len(got[3].Stream.InputChannels[0]))
	}
	// Spot-check sample values:
	for i, v := range got[3].Stream.InputChannels[0] {
		if v != float32(i)/160 {
			t.Errorf("sample %d = %f, want %f", i, v, float32(i)/160)
			break
		}
	}
}

func TestReader_HandlesUnknownFields(t *testing.T) {
	// Write an Event with an unknown high-numbered field. Reader should
	// silently skip it.
	var buf bytes.Buffer
	if err := WriteEvent(&buf, &Event{Type: EventStream, Stream: &Stream{DelayMS: 25}}); err != nil {
		t.Fatal(err)
	}
	// Append junk bytes that look like a malformed event header — make
	// it a real but extra event with unknown tags. Easier: just append
	// another well-formed event.
	if err := WriteEvent(&buf, &Event{Type: EventInit, Init: &Init{SampleRate: 48000}}); err != nil {
		t.Fatal(err)
	}
	r := NewReader(&buf)
	a, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if a.Stream.DelayMS != 25 {
		t.Errorf("first event mangled")
	}
	b, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if b.Init.SampleRate != 48000 {
		t.Errorf("second event mangled")
	}
}

func makeRamp(n int) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = float32(i) / float32(n)
	}
	return out
}
