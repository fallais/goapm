package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Cell is one row of the QA matrix: a single scenario configuration to
// evaluate. The runner produces one Score per Cell.
type Cell struct {
	ClipPath  string  `json:"clip"`
	NoisePath string  `json:"noise,omitempty"`
	IRPath    string  `json:"ir,omitempty"`
	SNRDB     float64 `json:"snr_db,omitempty"`
	Scenario  string  `json:"scenario"`
}

// Manifest mirrors testdata/*/manifest.yaml. Only the fields we need to
// drive the matrix are decoded; everything else is ignored.
type Manifest struct {
	Files []ManifestEntry `yaml:"files"`
}

type ManifestEntry struct {
	Name     string  `yaml:"name"`
	Duration float64 `yaml:"duration_s,omitempty"`
	Speaker  string  `yaml:"speaker,omitempty"`
	Notes    string  `yaml:"notes,omitempty"`
}

func loadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &m, nil
}

func discoverWavs(dir string) ([]string, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".wav" {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

// buildMatrix enumerates the Cartesian product of clips × noise × IRs,
// constrained to a sensible subset of SNR points. When clips or noise are
// missing the matrix degrades gracefully so the runner can still execute
// on whatever fixtures are present.
func buildMatrix(clipsDir, noiseDir, irsDir string) ([]Cell, error) {
	clips, err := discoverWavs(clipsDir)
	if err != nil {
		return nil, err
	}
	noises, err := discoverWavs(noiseDir)
	if err != nil {
		return nil, err
	}
	irs, err := discoverWavs(irsDir)
	if err != nil {
		return nil, err
	}
	snrs := []float64{0, 5, 10, 15}
	scenarios := []string{"ns_clean", "ns_noisy", "agc_target", "aec_echo"}

	var cells []Cell
	for _, sc := range scenarios {
		switch sc {
		case "ns_clean":
			for _, c := range clips {
				cells = append(cells, Cell{ClipPath: c, Scenario: sc})
			}
		case "ns_noisy":
			for _, c := range clips {
				for _, n := range noises {
					for _, s := range snrs {
						cells = append(cells, Cell{ClipPath: c, NoisePath: n, SNRDB: s, Scenario: sc})
					}
				}
			}
		case "agc_target":
			for _, c := range clips {
				cells = append(cells, Cell{ClipPath: c, Scenario: sc})
			}
		case "aec_echo":
			for _, c := range clips {
				for _, ir := range irs {
					cells = append(cells, Cell{ClipPath: c, IRPath: ir, Scenario: sc})
				}
			}
		}
	}
	return cells, nil
}
