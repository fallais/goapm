// Package window precomputes analysis/synthesis windows used by STFT-based
// modules (NS, AEC). All functions return freshly allocated slices —
// callers cache the result at module construction time.
package window

import "math"

// Hann returns an n-sample symmetric Hann window.
func Hann(n int) []float32 {
	w := make([]float32, n)
	if n == 1 {
		w[0] = 1
		return w
	}
	for i := 0; i < n; i++ {
		w[i] = float32(0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n-1)))
	}
	return w
}

// SqrtHann returns the square-root of an n-sample Hann window. Used by
// STFT pairs with 50 % hop so the analysis × synthesis product is Hann
// and overlap-add reconstructs perfectly.
func SqrtHann(n int) []float32 {
	w := Hann(n)
	for i, v := range w {
		w[i] = float32(math.Sqrt(float64(v)))
	}
	return w
}
