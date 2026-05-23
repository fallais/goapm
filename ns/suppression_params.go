package ns

type suppressionParams struct {
	overSubtractionFactor    float32
	minimumAttenuatingGain   float32
	useAttenuationAdjustment bool
}

func newSuppressionParams(level SuppressionLevel) suppressionParams {
	switch level {
	case Level6dB:
		return suppressionParams{
			overSubtractionFactor:    1.0,
			minimumAttenuatingGain:   0.5,
			useAttenuationAdjustment: false,
		}
	case Level12dB:
		return suppressionParams{
			overSubtractionFactor:    1.0,
			minimumAttenuatingGain:   0.25,
			useAttenuationAdjustment: true,
		}
	case Level18dB:
		return suppressionParams{
			overSubtractionFactor:    1.1,
			minimumAttenuatingGain:   0.125,
			useAttenuationAdjustment: true,
		}
	case Level21dB:
		return suppressionParams{
			overSubtractionFactor:    1.25,
			minimumAttenuatingGain:   0.09,
			useAttenuationAdjustment: true,
		}
	default:
		return suppressionParams{
			overSubtractionFactor:    1.0,
			minimumAttenuatingGain:   0.25,
			useAttenuationAdjustment: true,
		}
	}
}
