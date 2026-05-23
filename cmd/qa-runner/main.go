// Command qa-runner evaluates a matrix of clip × noise × IR scenarios
// against the gopam APM Processor and emits a JSON report. It is the
// gopam port of WebRTC's py_quality_assessment harness.
//
// Default mode is "smoke": runs against testdata/{clips,noise,irs}/smoke/
// and is expected to complete in seconds. Full-corpus mode pulls heavier
// data via testdata/*/pull.sh first.
//
// Usage:
//
//	qa-runner -out report.json
//	qa-runner -corpus full -out report.json
//	qa-runner -corpus synthetic -out report.json   # for CI without fixtures
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"time"

	"github.com/fallais/gopam/audio/wav"
	"github.com/fallais/gopam/test/synth"
)

func main() {
	corpus := flag.String("corpus", "smoke", "corpus: smoke | full | synthetic")
	out := flag.String("out", "qa-report.json", "output JSON report path")
	root := flag.String("testdata", "", "override testdata/ root (default: repo testdata)")
	flag.Parse()

	if err := runMain(*corpus, *out, *root); err != nil {
		log.Fatalf("qa-runner: %v", err)
	}
}

func runMain(corpus, outPath, rootOverride string) error {
	root := rootOverride
	if root == "" {
		var err error
		root, err = locateTestdata()
		if err != nil {
			return err
		}
	}
	subdir := corpus
	if corpus == "synthetic" {
		dir, err := materializeSynthetic()
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		root = dir
		subdir = ""
	}

	clipsDir := filepath.Join(root, "clips", subdir)
	noiseDir := filepath.Join(root, "noise", subdir)
	irsDir := filepath.Join(root, "irs", subdir)
	if subdir == "" {
		clipsDir = filepath.Join(root, "clips")
		noiseDir = filepath.Join(root, "noise")
		irsDir = filepath.Join(root, "irs")
	}

	cells, err := buildMatrix(clipsDir, noiseDir, irsDir)
	if err != nil {
		return err
	}
	log.Printf("qa-runner: %d cells from %s/{clips,noise,irs}%s", len(cells), root, subOrEmpty(subdir))

	start := time.Now()
	scores := make([]Score, len(cells))
	for i, c := range cells {
		scores[i] = scoreCell(c)
	}
	report := Report{
		Generated: time.Now().UTC(),
		Corpus:    corpus,
		Cells:     len(cells),
		Scores:    scores,
		ElapsedMS: time.Since(start).Milliseconds(),
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&report); err != nil {
		return err
	}
	log.Printf("qa-runner: wrote %s (%d cells, %d ms)", outPath, len(scores), report.ElapsedMS)
	return nil
}

func subOrEmpty(s string) string {
	if s == "" {
		return ""
	}
	return "/" + s
}

// Report is the top-level JSON document written by qa-runner.
type Report struct {
	Generated time.Time `json:"generated"`
	Corpus    string    `json:"corpus"`
	Cells     int       `json:"cells"`
	ElapsedMS int64     `json:"elapsed_ms"`
	Scores    []Score   `json:"scores"`
}

func locateTestdata() (string, error) {
	// Search upward from CWD for a "testdata" directory containing the
	// expected subdirs. This lets the runner work whether invoked from
	// the repo root or from inside cmd/qa-runner/.
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		td := filepath.Join(dir, "testdata")
		if hasSubdir(td, "clips") && hasSubdir(td, "noise") && hasSubdir(td, "irs") {
			return td, nil
		}
	}
	return "", fmt.Errorf("could not locate testdata/ from %s", cwd)
}

func hasSubdir(parent, name string) bool {
	info, err := os.Stat(filepath.Join(parent, name))
	return err == nil && info.IsDir()
}

// materializeSynthetic builds a throwaway corpus tree with a couple of
// generated WAV files. Used by CI smoke runs that need a guaranteed
// non-empty corpus regardless of git LFS state.
func materializeSynthetic() (string, error) {
	dir, err := os.MkdirTemp("", "gopam-synth-corpus-*")
	if err != nil {
		return "", err
	}
	rng := rand.New(rand.NewPCG(42, 42))

	makeDirOr := func(p string) error {
		return os.MkdirAll(p, 0o755)
	}
	clipsDir := filepath.Join(dir, "clips")
	noiseDir := filepath.Join(dir, "noise")
	irsDir := filepath.Join(dir, "irs")
	for _, d := range []string{clipsDir, noiseDir, irsDir} {
		if err := makeDirOr(d); err != nil {
			return "", err
		}
	}

	// Two short "clean speech" pretenders (sine + sweep) at 16 kHz.
	clip1 := synth.Sweep(16000, 16000, 200, 3000, 0.4)
	clip2 := synth.Sine(16000, 16000, 500, 0.4)
	if err := wav.WriteAll(filepath.Join(clipsDir, "sweep_200_3000.wav"), [][]float32{clip1}, 16000, wav.Float32); err != nil {
		return "", err
	}
	if err := wav.WriteAll(filepath.Join(clipsDir, "sine_500.wav"), [][]float32{clip2}, 16000, wav.Float32); err != nil {
		return "", err
	}

	// One noise sample (speech-shaped) at 16 kHz.
	noise := synth.SpeechShapedNoise(32000, 16000, rng)
	if err := wav.WriteAll(filepath.Join(noiseDir, "ssn.wav"), [][]float32{noise}, 16000, wav.Float32); err != nil {
		return "", err
	}

	// One IR (exponential decay, 150 ms RT60).
	ir := synth.ExpDecayIR(800, 16000, 0.15, rng)
	if err := wav.WriteAll(filepath.Join(irsDir, "decay_150ms.wav"), [][]float32{ir}, 16000, wav.Float32); err != nil {
		return "", err
	}
	return dir, nil
}
