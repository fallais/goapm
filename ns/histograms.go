package ns

const kHistogramSize = 1000

type histograms struct {
	lrt              [kHistogramSize]int
	spectralFlatness [kHistogramSize]int
	spectralDiff     [kHistogramSize]int
}

func (h *histograms) clear() {
	h.lrt = [kHistogramSize]int{}
	h.spectralFlatness = [kHistogramSize]int{}
	h.spectralDiff = [kHistogramSize]int{}
}

func (h *histograms) update(f *signalModel) {
	const oneByBinSizeLrt = 1.0 / binSizeLrt
	if f.lrt < kHistogramSize*binSizeLrt && f.lrt >= 0 {
		h.lrt[int(oneByBinSizeLrt*f.lrt)]++
	}

	const oneByBinSizeSpecFlat = 1.0 / binSizeSpecFlat
	if f.spectralFlatness < kHistogramSize*binSizeSpecFlat && f.spectralFlatness >= 0 {
		h.spectralFlatness[int(f.spectralFlatness*oneByBinSizeSpecFlat)]++
	}

	const oneByBinSizeSpecDiff = 1.0 / binSizeSpecDiff
	if f.spectralDiff < kHistogramSize*binSizeSpecDiff && f.spectralDiff >= 0 {
		h.spectralDiff[int(f.spectralDiff*oneByBinSizeSpecDiff)]++
	}
}
