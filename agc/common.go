package agc

const (
	kMinFloatS16Value    = -32768.0
	kMaxFloatS16Value    = 32767.0
	kMaxAbsFloatS16Value = 32768.0

	kMinLevelDbfs = -90.31

	kFrameDurationMs           = 10
	kSubFramesInFrame          = 20
	kMaxSamplesPerChannel      = 480
	kInputLevelScalingFactor   = 32768.0
	kMaxInputLevelLinear       = 36766.300710566735

	kInterpolatedGainCurveKneePoints       = 22
	kInterpolatedGainCurveBeyondKneePoints = 10
	kInterpolatedGainCurveTotalPoints      = kInterpolatedGainCurveKneePoints + kInterpolatedGainCurveBeyondKneePoints
)
