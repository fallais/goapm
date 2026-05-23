package ns

import "github.com/fallais/gopam/dsp/fft"

// fftWrapper exposes the kFftSize FFT used by NS using the gopam dsp/fft
// plan. Mirrors upstream NrFft.
type fftWrapper struct {
	plan *fft.Plan
}

func newFFT() *fftWrapper { return &fftWrapper{plan: fft.New(fftSize)} }

// forward computes |X|+jY from the time-domain frame. Output arrays are
// of length fftSizeBy2Plus1, with imag[0] = imag[N/2] = 0.
func (f *fftWrapper) forward(timeData *[fftSize]float32, real, imag *[fftSizeBy2Plus1]float32) {
	f.plan.Forward(timeData[:], real[:], imag[:])
}

// inverse reconstructs the time-domain frame from the half spectrum. Out
// length is fftSize. The dsp/fft Inverse normalizes by 1/N which matches
// the math; no extra 2/N step is needed.
func (f *fftWrapper) inverse(real, imag *[fftSizeBy2Plus1]float32, timeData *[fftSize]float32) {
	f.plan.Inverse(real[:], imag[:], timeData[:])
}
