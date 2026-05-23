package apm

// Config selects which modules are active in the Processor pipeline and
// their per-module settings. Zero value means every module disabled.
type Config struct {
	HPF HPFConfig
	NS  NSConfig
	AGC AGCConfig
	AEC AECConfig
}

// HPFConfig controls the high-pass filter (DC blocker + rumble removal).
type HPFConfig struct {
	Enabled bool
}

// NSConfig controls the noise suppressor.
type NSConfig struct {
	Enabled bool
	Level   NSLevel
}

// NSLevel selects the noise-suppression aggressiveness.
type NSLevel int

const (
	NSLow NSLevel = iota
	NSModerate
	NSHigh
	NSVeryHigh
)

// AGCConfig controls the adaptive digital automatic gain control.
type AGCConfig struct {
	Enabled         bool
	TargetLevelDBFS float32 // typical: -3 dBFS
	CompressionGain float32 // dB, typical: 9
}

// AECConfig controls the acoustic echo canceller.
type AECConfig struct {
	Enabled bool
}

// DefaultConfig returns a config with sensible defaults but every module
// disabled. Enable selectively by setting the per-module Enabled field.
func DefaultConfig() Config {
	return Config{
		NS: NSConfig{Level: NSModerate},
		AGC: AGCConfig{
			TargetLevelDBFS: -3,
			CompressionGain: 9,
		},
	}
}
