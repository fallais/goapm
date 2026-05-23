// Package visqol wraps the ViSQOL external binary to score speech and
// audio quality. ViSQOL is an open-source perceptual quality estimator
// from Google: https://github.com/google/visqol
//
// gopam does not bundle ViSQOL. Tests that depend on it should check
// Available() first and skip when the binary is missing.
//
// Build/install ViSQOL from source, then make `visqol` available on PATH
// (or set GOPAM_VISQOL to an absolute path).
package visqol

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// PinnedVersion is the ViSQOL upstream tag we test against. Bump in
// lockstep with our calibration of expected MOS-LQO ranges.
const PinnedVersion = "v3.3.3"

// Mode selects the ViSQOL operating mode.
type Mode int

const (
	// SpeechMode is 16 kHz narrowband speech (no spectrogram alignment).
	SpeechMode Mode = iota
	// AudioMode is 48 kHz wideband audio (default ViSQOL behavior).
	AudioMode
)

// Result is a parsed ViSQOL score.
type Result struct {
	Reference string
	Degraded  string
	MOSLQO    float64
}

// Binary returns the resolved path to the ViSQOL executable, honoring
// GOPAM_VISQOL if set.
func Binary() string {
	if p := os.Getenv("GOPAM_VISQOL"); p != "" {
		return p
	}
	return "visqol"
}

// Available reports whether a ViSQOL binary is reachable.
func Available() bool {
	_, err := exec.LookPath(Binary())
	return err == nil
}

// Score runs ViSQOL on a reference/degraded WAV pair and returns the
// parsed MOS-LQO score. Both files must be the same sample rate; matching
// to ViSQOL's expected rate (16 kHz for speech, 48 kHz for audio) is the
// caller's responsibility.
func Score(ctx context.Context, mode Mode, referenceWAV, degradedWAV string) (*Result, error) {
	if !Available() {
		return nil, errors.New("visqol: binary not found on PATH (install it or set GOPAM_VISQOL)")
	}
	args := []string{
		"--reference_file", referenceWAV,
		"--degraded_file", degradedWAV,
	}
	if mode == SpeechMode {
		args = append(args, "--use_speech_mode")
	}
	cmd := exec.CommandContext(ctx, Binary(), args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("visqol failed: %w; stderr=%s", err, string(ee.Stderr))
		}
		return nil, fmt.Errorf("visqol failed: %w", err)
	}
	return parseOutput(string(out), referenceWAV, degradedWAV)
}

// ScoreBatch runs ViSQOL once over a CSV manifest of reference/degraded
// pairs and returns one Result per row. The manifest format is the one
// ViSQOL itself accepts: two columns, header "reference,degraded".
func ScoreBatch(ctx context.Context, mode Mode, manifestCSV, resultsCSV string) ([]Result, error) {
	if !Available() {
		return nil, errors.New("visqol: binary not found")
	}
	args := []string{
		"--batch_input_csv", manifestCSV,
		"--results_csv", resultsCSV,
	}
	if mode == SpeechMode {
		args = append(args, "--use_speech_mode")
	}
	cmd := exec.CommandContext(ctx, Binary(), args...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("visqol batch failed: %w", err)
	}
	f, err := os.Open(resultsCSV)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseResultsCSV(f)
}

// parseOutput interprets ViSQOL's stdout. The binary prints a banner and
// then "MOS-LQO: <value>" — we tolerate slight format variations.
func parseOutput(s, ref, deg string) (*Result, error) {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		const prefix = "MOS-LQO:"
		if i := strings.Index(line, prefix); i >= 0 {
			v, err := strconv.ParseFloat(strings.TrimSpace(line[i+len(prefix):]), 64)
			if err != nil {
				return nil, fmt.Errorf("visqol: malformed score line %q: %w", line, err)
			}
			return &Result{Reference: ref, Degraded: deg, MOSLQO: v}, nil
		}
	}
	return nil, fmt.Errorf("visqol: no MOS-LQO line in output:\n%s", s)
}

func parseResultsCSV(r io.Reader) ([]Result, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("visqol: empty results CSV")
	}
	header := rows[0]
	refIdx, degIdx, scoreIdx := -1, -1, -1
	for i, h := range header {
		switch strings.ToLower(h) {
		case "reference":
			refIdx = i
		case "degraded":
			degIdx = i
		case "moslqo", "mos-lqo", "moslqo_score":
			scoreIdx = i
		}
	}
	if refIdx < 0 || degIdx < 0 || scoreIdx < 0 {
		return nil, fmt.Errorf("visqol: unexpected results columns: %v", header)
	}
	out := make([]Result, 0, len(rows)-1)
	for _, row := range rows[1:] {
		v, err := strconv.ParseFloat(row[scoreIdx], 64)
		if err != nil {
			return nil, err
		}
		out = append(out, Result{Reference: row[refIdx], Degraded: row[degIdx], MOSLQO: v})
	}
	return out, nil
}
