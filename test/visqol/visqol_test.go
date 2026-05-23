package visqol

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fallais/gopam/audio/wav"
	"github.com/fallais/gopam/test/synth"
)

func TestAvailable_GuardsTests(t *testing.T) {
	if !Available() {
		t.Skip("ViSQOL not on PATH; install per test/visqol/doc.go")
	}
}

func TestScore_IdenticalFiles(t *testing.T) {
	if !Available() {
		t.Skip("ViSQOL not on PATH")
	}
	dir := t.TempDir()
	clean := synth.Sine(48000, 48000, 1000, 0.5)
	a := filepath.Join(dir, "a.wav")
	b := filepath.Join(dir, "b.wav")
	if err := wav.WriteAll(a, [][]float32{clean}, 48000, wav.PCM16); err != nil {
		t.Fatal(err)
	}
	if err := wav.WriteAll(b, [][]float32{clean}, 48000, wav.PCM16); err != nil {
		t.Fatal(err)
	}
	r, err := Score(context.Background(), AudioMode, a, b)
	if err != nil {
		t.Fatal(err)
	}
	if r.MOSLQO < 4.0 {
		t.Errorf("identical WAV pair should score high; got %f", r.MOSLQO)
	}
}

func TestParseOutput_PicksScore(t *testing.T) {
	out := `Visqol version 3.3.3
some banner
MOS-LQO: 4.521
done.`
	r, err := parseOutput(out, "a.wav", "b.wav")
	if err != nil {
		t.Fatal(err)
	}
	if r.MOSLQO != 4.521 {
		t.Errorf("got %f", r.MOSLQO)
	}
}
