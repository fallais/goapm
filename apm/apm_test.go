package apm

import "testing"

func TestPassthrough_PreservesSamples(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	f := NewFrame(Rate16k, 1)
	for i := range f.Data[0] {
		f.Data[0][i] = float32(i) / 160
	}
	want := append([]float32(nil), f.Data[0]...)

	if err := p.ProcessStream(f); err != nil {
		t.Fatal(err)
	}
	for i, got := range f.Data[0] {
		if got != want[i] {
			t.Fatalf("sample %d: passthrough should leave samples untouched, got %f want %f", i, got, want[i])
		}
	}
}

func TestFrame_Validate(t *testing.T) {
	tests := []struct {
		name    string
		frame   *Frame
		wantErr bool
	}{
		{"good 16k mono", NewFrame(Rate16k, 1), false},
		{"good 48k stereo", NewFrame(Rate48k, 2), false},
		{"bad rate", &Frame{SampleRate: 12345, Data: [][]float32{make([]float32, 160)}}, true},
		{"no channels", &Frame{SampleRate: Rate16k}, true},
		{"short channel", &Frame{SampleRate: Rate16k, Data: [][]float32{make([]float32, 100)}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.frame.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSamplesPerFrame(t *testing.T) {
	cases := []struct {
		rate SampleRate
		want int
	}{
		{Rate8k, 80},
		{Rate16k, 160},
		{Rate32k, 320},
		{Rate48k, 480},
	}
	for _, c := range cases {
		if got := c.rate.SamplesPerFrame(); got != c.want {
			t.Errorf("Rate%d.SamplesPerFrame() = %d, want %d", c.rate, got, c.want)
		}
	}
}

func TestProcessor_ZeroAllocSteadyState(t *testing.T) {
	p, _ := New(DefaultConfig())
	f := NewFrame(Rate16k, 1)
	// Warm up — first call may allocate scratch.
	_ = p.ProcessStream(f)
	allocs := testing.AllocsPerRun(1000, func() {
		_ = p.ProcessStream(f)
	})
	if allocs != 0 {
		t.Errorf("ProcessStream allocates in steady state: %.1f allocs/op (want 0)", allocs)
	}
}
