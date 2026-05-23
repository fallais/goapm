package ns

const (
	fftSize         = 256
	fftSizeBy2Plus1 = fftSize/2 + 1
	nsFrameSize     = 160
	overlapSize     = fftSize - nsFrameSize

	shortStartupPhaseBlocks = 50
	longStartupPhaseBlocks  = 200
	featureUpdateWindowSize = 500

	ltrFeatureThr    = 0.5
	binSizeLrt       = 0.1
	binSizeSpecFlat  = 0.05
	binSizeSpecDiff  = 0.1
)
