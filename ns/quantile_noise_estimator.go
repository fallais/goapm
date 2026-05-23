package ns

import "math"

const kSimult = 3

type quantileNoiseEstimator struct {
	density     [kSimult * fftSizeBy2Plus1]float32
	logQuantile [kSimult * fftSizeBy2Plus1]float32
	quantile    [fftSizeBy2Plus1]float32
	counter     [kSimult]int
	numUpdates  int
}

func newQuantileNoiseEstimator() *quantileNoiseEstimator {
	q := &quantileNoiseEstimator{numUpdates: 1}
	for i := range q.density {
		q.density[i] = 0.3
	}
	for i := range q.logQuantile {
		q.logQuantile[i] = 8.0
	}
	const oneBySimult = 1.0 / float64(kSimult)
	for i := 0; i < kSimult; i++ {
		q.counter[i] = int(math.Floor(float64(longStartupPhaseBlocks) * float64(i+1) * oneBySimult))
	}
	return q
}

func (q *quantileNoiseEstimator) estimate(signalSpectrum *[fftSizeBy2Plus1]float32, noiseSpectrum *[fftSizeBy2Plus1]float32) {
	var logSpectrum [fftSizeBy2Plus1]float32
	logApproxSpan(signalSpectrum[:], logSpectrum[:])

	quantileIndexToReturn := -1
	for s, k := 0, 0; s < kSimult; s, k = s+1, k+fftSizeBy2Plus1 {
		oneByCounterPlus1 := 1.0 / float32(q.counter[s]+1)
		for i, j := 0, k; i < fftSizeBy2Plus1; i, j = i+1, j+1 {
			var delta float32 = 40.0
			if q.density[j] > 1.0 {
				delta = 40.0 / q.density[j]
			}
			multiplier := delta * oneByCounterPlus1
			if logSpectrum[i] > q.logQuantile[j] {
				q.logQuantile[j] += 0.25 * multiplier
			} else {
				q.logQuantile[j] -= 0.75 * multiplier
			}

			const kWidth = float32(0.01)
			const kOneByWidthPlus2 = 1.0 / (2.0 * kWidth)
			d := logSpectrum[i] - q.logQuantile[j]
			if d < 0 {
				d = -d
			}
			if d < kWidth {
				q.density[j] = (float32(q.counter[s])*q.density[j] + kOneByWidthPlus2) * oneByCounterPlus1
			}
		}

		if q.counter[s] >= longStartupPhaseBlocks {
			q.counter[s] = 0
			if q.numUpdates >= longStartupPhaseBlocks {
				quantileIndexToReturn = k
			}
		}
		q.counter[s]++
	}

	if q.numUpdates < longStartupPhaseBlocks {
		quantileIndexToReturn = fftSizeBy2Plus1 * (kSimult - 1)
		q.numUpdates++
	}

	if quantileIndexToReturn >= 0 {
		expApproxSpan(q.logQuantile[quantileIndexToReturn:quantileIndexToReturn+fftSizeBy2Plus1], q.quantile[:])
	}

	*noiseSpectrum = q.quantile
}
