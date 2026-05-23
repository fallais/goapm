package agc

// fixedDigitalLevelEstimator: smooth envelope over a 10 ms frame split into
// 20 sub-frames, instant attack, exponential decay (kDecayMs = 20).
// Mirrors fixed_digital_level_estimator.{h,cc}.
type fixedDigitalLevelEstimator struct {
	filterStateLevel  float32
	samplesInFrame    int
	samplesInSubFrame int
}

const (
	attackFilterConstant = 0.0
	decayFilterConstant  = 0.9971259
)

func newLevelEstimator(samplesPerChannel int) *fixedDigitalLevelEstimator {
	e := &fixedDigitalLevelEstimator{}
	e.setSamplesPerChannel(samplesPerChannel)
	return e
}

func (e *fixedDigitalLevelEstimator) setSamplesPerChannel(s int) {
	e.samplesInFrame = s
	e.samplesInSubFrame = s / kSubFramesInFrame
}

func (e *fixedDigitalLevelEstimator) reset() { e.filterStateLevel = 0 }

func (e *fixedDigitalLevelEstimator) lastAudioLevel() float32 { return e.filterStateLevel }

// computeLevel returns one envelope value per sub-frame (20 values).
func (e *fixedDigitalLevelEstimator) computeLevel(channels [][]float32, envelope *[kSubFramesInFrame]float32) {
	*envelope = [kSubFramesInFrame]float32{}
	for _, ch := range channels {
		for sf := 0; sf < kSubFramesInFrame; sf++ {
			base := sf * e.samplesInSubFrame
			for i := 0; i < e.samplesInSubFrame; i++ {
				v := ch[base+i]
				if v < 0 {
					v = -v
				}
				if v > envelope[sf] {
					envelope[sf] = v
				}
			}
		}
	}

	for sf := 0; sf < kSubFramesInFrame-1; sf++ {
		if envelope[sf] < envelope[sf+1] {
			envelope[sf] = envelope[sf+1]
		}
	}

	for sf := 0; sf < kSubFramesInFrame; sf++ {
		v := envelope[sf]
		if v > e.filterStateLevel {
			envelope[sf] = v*(1-attackFilterConstant) + e.filterStateLevel*attackFilterConstant
		} else {
			envelope[sf] = v*(1-decayFilterConstant) + e.filterStateLevel*decayFilterConstant
		}
		e.filterStateLevel = envelope[sf]
	}
}
