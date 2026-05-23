package wav

import (
	"math"
	"path/filepath"
	"testing"
)

func TestRoundTrip_PCM16(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt.wav")

	const n = 480 // 10 ms at 48 kHz
	src := make([]float32, n)
	for i := range src {
		src[i] = float32(math.Sin(2 * math.Pi * 1000 * float64(i) / 48000))
	}
	if err := WriteAll(path, [][]float32{src}, 48000, PCM16); err != nil {
		t.Fatal(err)
	}

	chans, rate, err := ReadAll(path)
	if err != nil {
		t.Fatal(err)
	}
	if rate != 48000 {
		t.Fatalf("rate %d, want 48000", rate)
	}
	if len(chans) != 1 || len(chans[0]) != n {
		t.Fatalf("shape %dx%d, want 1x%d", len(chans), len(chans[0]), n)
	}
	// PCM16 quantization noise is bounded by 1/32768.
	const tol = 1.0 / 32768
	for i, got := range chans[0] {
		if math.Abs(float64(got-src[i])) > tol*2 {
			t.Fatalf("sample %d: got %f want %f (tol %f)", i, got, src[i], tol)
		}
	}
}

func TestRoundTrip_Float32(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rtf.wav")

	const n = 480
	src := make([]float32, n)
	for i := range src {
		src[i] = float32(math.Sin(2 * math.Pi * 440 * float64(i) / 48000))
	}
	if err := WriteAll(path, [][]float32{src}, 48000, Float32); err != nil {
		t.Fatal(err)
	}
	chans, _, err := ReadAll(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, got := range chans[0] {
		if got != src[i] {
			t.Fatalf("float32 should be exact: i=%d got=%f want=%f", i, got, src[i])
		}
	}
}

func TestRoundTrip_Stereo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stereo.wav")

	const n = 160
	left := make([]float32, n)
	right := make([]float32, n)
	for i := range left {
		left[i] = float32(i) / n
		right[i] = -float32(i) / n
	}
	if err := WriteAll(path, [][]float32{left, right}, 16000, PCM16); err != nil {
		t.Fatal(err)
	}
	chans, rate, err := ReadAll(path)
	if err != nil {
		t.Fatal(err)
	}
	if rate != 16000 || len(chans) != 2 {
		t.Fatalf("rate=%d chans=%d", rate, len(chans))
	}
	if len(chans[0]) != n || len(chans[1]) != n {
		t.Fatalf("channel lengths: %d %d", len(chans[0]), len(chans[1]))
	}
}

func TestPCM16_ClipsOverrange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.wav")
	if err := WriteAll(path, [][]float32{{2.0, -2.0, 0.5, -0.5}}, 16000, PCM16); err != nil {
		t.Fatal(err)
	}
	chans, _, err := ReadAll(path)
	if err != nil {
		t.Fatal(err)
	}
	got := chans[0]
	// Clipped values round to ±1.0 (within 1 LSB).
	if math.Abs(float64(got[0]-1.0)) > 1.0/32768*2 {
		t.Errorf("overrange positive: got %f", got[0])
	}
	if math.Abs(float64(got[1]-(-1.0))) > 1.0/32768*2 {
		t.Errorf("overrange negative: got %f", got[1])
	}
}
