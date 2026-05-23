# gopam

A Go port of WebRTC's Audio Processing Module (APM): acoustic echo
cancellation, noise suppression, automatic gain control, high-pass filter.

## Layout

| Package | Purpose |
| --- | --- |
| `apm` | Public Processor — composes the pipeline modules per stream |
| `hpf` | High-pass filter (3-stage cascaded biquad per rate) |
| `ns` | Noise suppression (STFT Wiener filter with MCRA noise tracking) |
| `agc` | Automatic gain control (fixed-digital gain + limiter) |
| `dsp/fft` | Allocation-free real radix-2 FFT |
| `dsp/biquad` | Direct-form I cascaded biquad |
| `dsp/window` | Hann / sqrt-Hann tables |
| `audio/wav` | Streaming WAV I/O |
| `test/synth` | Synthetic signal generators |
| `test/metrics` | DSP metrics (segSNR, LSD, cepstral, ERLE, attack time) |
| `test/visqol` | External ViSQOL binary wrapper |
| `test/property` | Property-based scenarios with thresholded assertions |
| `test/aecdump` | Replay of WebRTC `.aecdump` debug recordings |
| `test/bench` | Per-frame benchmarks + real-time-factor reporter |
| `cmd/aecdump-replay` | Drive a Processor with an `.aecdump`, emit WAV |
| `cmd/qa-runner` | Matrix of clip × noise × IR scenarios → JSON report |
| `cmd/rtf-bench` | Standalone real-time-factor measurement |

The port tracks upstream WebRTC's `modules/audio_processing/` at the pinned
commit in `third_party/webrtc-proto/PINNED_COMMIT`.

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
are skipped when the binary is absent. To enable, build ViSQOL from
[google/visqol](https://github.com/google/visqol) and place the binary on
PATH. The version we pin against is recorded in `test/visqol/doc.go`.

## License

Apache 2.0. See [LICENSE](LICENSE).

Portions of this project port logic from WebRTC, which is licensed under
the BSD 3-Clause License. Upstream notices are preserved under
`third_party/webrtc-proto/`.
