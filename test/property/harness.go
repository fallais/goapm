// Package property holds property-based test scenarios that assert DSP
// behavior (ERLE thresholds, SNR-improvement targets, attack times) on
// each APM module. Scenarios skip themselves until the module they
// exercise is implemented — see Implemented.
//
// Thresholds for every scenario live in thresholds.yaml at this package
// root; LoadThresholds parses it once per test binary.
package property

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/fallais/gopam/apm"
)

// Module names tracked by the implementation registry.
const (
	ModuleHPF = "hpf"
	ModuleNS  = "ns"
	ModuleAGC = "agc"
	ModuleAEC = "aec"
)

// Implemented returns whether the named module currently performs real
// processing (vs. passthrough). Update this map as modules land.
//
// Property tests that need the module to actually work call
// RequireImplemented to skip themselves until the module ships.
var Implemented = map[string]bool{
	ModuleHPF: false,
	ModuleNS:  false,
	ModuleAGC: false,
	ModuleAEC: false,
}

// RequireImplemented skips the test if the named module is still a
// passthrough stub. T must satisfy a minimal Skipf interface (testing.T).
type Skipper interface {
	Skipf(format string, args ...any)
	Helper()
}

func RequireImplemented(t Skipper, module string) {
	t.Helper()
	if !Implemented[module] {
		t.Skipf("module %q not implemented yet (still passthrough)", module)
	}
}

// Thresholds is the parsed contents of thresholds.yaml: scenario name →
// expected metric bounds. Tests query it to keep the spec out of code.
type Thresholds map[string]ScenarioBounds

// ScenarioBounds describes pass/fail bounds for one named scenario.
type ScenarioBounds struct {
	MinERLE       *float64 `yaml:"min_erle_db,omitempty"`
	MinSNRGain    *float64 `yaml:"min_snr_gain_db,omitempty"`
	MaxLSD        *float64 `yaml:"max_lsd_db,omitempty"`
	MaxCepstralCD *float64 `yaml:"max_cepstral_db,omitempty"`
	MaxAttackS    *float64 `yaml:"max_attack_s,omitempty"`
	MaxPeakDBFS   *float64 `yaml:"max_peak_dbfs,omitempty"`
	MinCornerDB   *float64 `yaml:"min_corner_db,omitempty"`
	Notes         string   `yaml:"notes,omitempty"`
}

var (
	thresholdsOnce sync.Once
	thresholds     Thresholds
	thresholdsErr  error
)

// LoadThresholds parses thresholds.yaml relative to this package's
// directory. Cached on first call.
func LoadThresholds() (Thresholds, error) {
	thresholdsOnce.Do(func() {
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			thresholdsErr = errors.New("property: cannot resolve thresholds.yaml path")
			return
		}
		path := filepath.Join(filepath.Dir(file), "thresholds.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			thresholdsErr = fmt.Errorf("property: read thresholds: %w", err)
			return
		}
		if err := yaml.Unmarshal(data, &thresholds); err != nil {
			thresholdsErr = fmt.Errorf("property: parse thresholds: %w", err)
			return
		}
	})
	return thresholds, thresholdsErr
}

// MustBounds fetches the bounds for the named scenario or fails the test.
type Fataler interface {
	Fatalf(format string, args ...any)
	Helper()
}

func MustBounds(t Fataler, scenario string) ScenarioBounds {
	t.Helper()
	th, err := LoadThresholds()
	if err != nil {
		t.Fatalf("loading thresholds: %v", err)
	}
	b, ok := th[scenario]
	if !ok {
		t.Fatalf("no thresholds entry for scenario %q", scenario)
	}
	return b
}

// Pipeline is the bridge between synthesized planar signals and the APM
// Processor (which works on Frames). It splits a long buffer into
// 10 ms frames and calls ProcessStream on each.
//
// Frames are reused across calls — zero allocations in the hot loop once
// Process is warm.
type Pipeline struct {
	processor *apm.Processor
	rate      apm.SampleRate
	frame     *apm.Frame
}

// NewPipeline wraps a Processor for batch processing of a single-channel
// stream at the given sample rate.
func NewPipeline(p *apm.Processor, rate apm.SampleRate) *Pipeline {
	return &Pipeline{processor: p, rate: rate, frame: apm.NewFrame(rate, 1)}
}

// ProcessStream runs near-end samples through the pipeline 10 ms at a
// time. Output is written into outBuf (which may be the same slice as
// near). Returns the number of samples processed.
func (p *Pipeline) ProcessStream(near, outBuf []float32) (int, error) {
	if len(outBuf) < len(near) {
		return 0, fmt.Errorf("property: outBuf shorter than input")
	}
	per := p.rate.SamplesPerFrame()
	n := (len(near) / per) * per
	for i := 0; i < n; i += per {
		copy(p.frame.Data[0], near[i:i+per])
		if err := p.processor.ProcessStream(p.frame); err != nil {
			return i, err
		}
		copy(outBuf[i:i+per], p.frame.Data[0])
	}
	return n, nil
}

// ProcessReverseStream pushes far-end samples through the reverse stream.
// Length must be a multiple of one 10 ms frame.
func (p *Pipeline) ProcessReverseStream(far []float32) error {
	per := p.rate.SamplesPerFrame()
	for i := 0; i+per <= len(far); i += per {
		copy(p.frame.Data[0], far[i:i+per])
		if err := p.processor.ProcessReverseStream(p.frame); err != nil {
			return err
		}
	}
	return nil
}
