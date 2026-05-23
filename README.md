# gopam

A Go port of WebRTC's Audio Processing Module (APM): acoustic echo
cancellation, noise suppression, automatic gain control, high-pass filter,
voice activity detection.

**Status:** test infrastructure only. No DSP modules are implemented yet.
The `apm.Processor` is a passthrough stub until module porting begins.

## What ships today

This repository currently contains the testing apparatus, modeled on
Google's own APM testing stack:

| Component | Purpose |
| --- | --- |
| `apm` | Public API surface (passthrough Processor for now) |
| `audio/wav` | Streaming WAV I/O |
| `test/synth` | Synthetic signal generators (tones, noise, impulses, IRs) |
| `test/metrics` | Native DSP metrics (segSNR, LSD, cepstral, ERLE, attack time) |
| `test/visqol` | Wrapper around the ViSQOL external binary for MOS-style scores |
| `test/property` | Property-based scenarios with thresholded assertions |
| `test/aecdump` | Replay of WebRTC `.aecdump` debug recordings |
| `test/bench` | Per-frame benchmarks + real-time-factor reporter |
| `cmd/aecdump-replay` | Replay an `.aecdump` through a Processor, emit WAV |
| `cmd/qa-runner` | Run a matrix of clip × noise × IR scenarios, emit JSON |
| `cmd/rtf-bench` | Standalone real-time-factor measurement |

## Quick start

```
go build ./...
go test ./...
go test -bench=. ./test/bench/...
```

## Pulling the full corpus

PR-time CI runs against the small smoke corpus committed in
`testdata/*/smoke/`. The full corpus is pulled on demand:

```
./testdata/clips/pull.sh
./testdata/noise/pull.sh
./testdata/irs/pull.sh
./cmd/qa-runner/qa-runner -corpus full -out report.json
```

## ViSQOL

ViSQOL is not required to run the test suite; metrics that depend on it
are skipped when the binary is absent. To enable:

```
# Build ViSQOL from https://github.com/google/visqol and place the
# binary on PATH, then:
visqol --version
```

The version we pin against is recorded in `test/visqol/doc.go`.

## Repository layout

See [docs/LAYOUT.md](docs/LAYOUT.md) for the full directory map and rationale.

## License

Apache 2.0. See [LICENSE](LICENSE).

This project intentionally tracks WebRTC's licensing (BSD 3-Clause for the
original code we draw inspiration from, Apache 2.0 for our own work) so that
later ports of upstream algorithms remain compatible.
