package ns

import "fmt"

type channelState struct {
	speechProb              *speechProbabilityEstimator
	wienerFilter            *wienerFilter
	noiseEstimator          *noiseEstimator
	prevAnalysisSignalSpec  [fftSizeBy2Plus1]float32
	analyzeAnalysisMemory   [overlapSize]float32
	processAnalysisMemory   [overlapSize]float32
	processSynthesisMemory  [overlapSize]float32
}

func newChannelState(sp *suppressionParams) *channelState {
	ne := newNoiseEstimator(sp)
	cs := &channelState{
		speechProb:     newSpeechProbabilityEstimator(),
		wienerFilter:   newWienerFilter(sp),
		noiseEstimator: ne,
	}
	for i := range cs.prevAnalysisSignalSpec {
		cs.prevAnalysisSignalSpec[i] = 1
	}
	return cs
}

type filterBankState struct {
	real          [fftSizeBy2Plus1]float32
	imag          [fftSizeBy2Plus1]float32
	extendedFrame [fftSize]float32
}

// Suppressor is the noise-suppression engine for a single stream. One
// instance per stream. Not safe for concurrent use.
type Suppressor struct {
	params              suppressionParams
	numAnalyzedFrames   int32
	fft                 *fftWrapper
	captureOutputUsed   bool
	channels            []*channelState
	filterBankStates    []filterBankState
	energiesBefore      []float32
	gainAdjustments     []float32
}

// NewSuppressor constructs a Suppressor. Only 16 kHz is supported; calls
// at other rates return an error.
func NewSuppressor(cfg Config, sampleRateHz, numChannels int) (*Suppressor, error) {
	if sampleRateHz != 16000 {
		return nil, fmt.Errorf("ns: only 16 kHz supported, got %d", sampleRateHz)
	}
	if numChannels <= 0 {
		return nil, fmt.Errorf("ns: numChannels must be > 0")
	}
	params := newSuppressionParams(cfg.TargetLevel)
	s := &Suppressor{
		params:            params,
		numAnalyzedFrames: -1,
		fft:               newFFT(),
		captureOutputUsed: true,
		channels:          make([]*channelState, numChannels),
		filterBankStates:  make([]filterBankState, numChannels),
		energiesBefore:    make([]float32, numChannels),
		gainAdjustments:   make([]float32, numChannels),
	}
	for c := range s.channels {
		s.channels[c] = newChannelState(&s.params)
	}
	return s, nil
}

// SetCaptureOutputUsed mirrors upstream's SetCaptureOutputUsage: when
// false, Process skips the synthesis step (the output isn't used).
func (s *Suppressor) SetCaptureOutputUsed(used bool) { s.captureOutputUsed = used }

// NumChannels returns the configured channel count.
func (s *Suppressor) NumChannels() int { return len(s.channels) }

// Analyze adapts the noise model. audio is [channel][sample] with 160
// samples per channel (one 10 ms frame at 16 kHz). The audio is read but
// not modified.
func (s *Suppressor) Analyze(audio [][]float32) error {
	if len(audio) != len(s.channels) {
		return fmt.Errorf("ns.Analyze: got %d channels, want %d", len(audio), len(s.channels))
	}
	for c := range audio {
		if len(audio[c]) != nsFrameSize {
			return fmt.Errorf("ns.Analyze: channel %d has %d samples, want %d", c, len(audio[c]), nsFrameSize)
		}
		s.channels[c].noiseEstimator.prepareAnalysis()
	}

	zeroFrame := true
	for c := range audio {
		if computeEnergyFrameAndMemory(audio[c], &s.channels[c].analyzeAnalysisMemory) > 0 {
			zeroFrame = false
			break
		}
	}
	if zeroFrame {
		return nil
	}

	s.numAnalyzedFrames++
	if s.numAnalyzedFrames < 0 {
		s.numAnalyzedFrames = 0
	}

	for c := range audio {
		ch := s.channels[c]
		var extended [fftSize]float32
		formExtendedFrame(audio[c], &ch.analyzeAnalysisMemory, &extended)
		applyFilterBankWindow(&extended)

		var real, imag [fftSizeBy2Plus1]float32
		s.fft.forward(&extended, &real, &imag)

		var signalSpec [fftSizeBy2Plus1]float32
		computeMagnitudeSpectrum(&real, &imag, &signalSpec)

		var signalEnergy float32
		for i := 0; i < fftSizeBy2Plus1; i++ {
			signalEnergy += real[i]*real[i] + imag[i]*imag[i]
		}
		signalEnergy /= float32(fftSizeBy2Plus1)

		var signalSpectralSum float32
		for i := 0; i < fftSizeBy2Plus1; i++ {
			signalSpectralSum += signalSpec[i]
		}

		ch.noiseEstimator.preUpdate(s.numAnalyzedFrames, &signalSpec, signalSpectralSum)

		var priorSnr, postSnr [fftSizeBy2Plus1]float32
		computeSnr(
			(*[fftSizeBy2Plus1]float32)(&ch.wienerFilter.filter),
			&ch.prevAnalysisSignalSpec,
			&signalSpec,
			&ch.noiseEstimator.prevNoiseSpec,
			&ch.noiseEstimator.noiseSpec,
			&priorSnr,
			&postSnr,
		)

		ch.speechProb.update(
			s.numAnalyzedFrames,
			&priorSnr,
			&postSnr,
			&ch.noiseEstimator.conservativeSpec,
			&signalSpec,
			signalSpectralSum,
			signalEnergy,
		)

		ch.noiseEstimator.postUpdate(&ch.speechProb.speechProb, &signalSpec)

		ch.prevAnalysisSignalSpec = signalSpec
	}
	return nil
}

// Process applies suppression in place. audio is [channel][sample] with
// 160 samples per channel. Output replaces input.
func (s *Suppressor) Process(audio [][]float32) error {
	if len(audio) != len(s.channels) {
		return fmt.Errorf("ns.Process: got %d channels, want %d", len(audio), len(s.channels))
	}
	for c := range audio {
		if len(audio[c]) != nsFrameSize {
			return fmt.Errorf("ns.Process: channel %d has %d samples, want %d", c, len(audio[c]), nsFrameSize)
		}
	}

	for c := range audio {
		fbs := &s.filterBankStates[c]
		ch := s.channels[c]
		formExtendedFrame(audio[c], &ch.processAnalysisMemory, &fbs.extendedFrame)
		applyFilterBankWindow(&fbs.extendedFrame)
		s.energiesBefore[c] = computeEnergyOfExtended(&fbs.extendedFrame)

		s.fft.forward(&fbs.extendedFrame, &fbs.real, &fbs.imag)

		var signalSpec [fftSizeBy2Plus1]float32
		computeMagnitudeSpectrum(&fbs.real, &fbs.imag, &signalSpec)

		ch.wienerFilter.update(
			s.numAnalyzedFrames,
			&ch.noiseEstimator.noiseSpec,
			&ch.noiseEstimator.prevNoiseSpec,
			&ch.noiseEstimator.parametricSpec,
			&signalSpec,
		)
	}

	if !s.captureOutputUsed {
		return nil
	}

	var filter [fftSizeBy2Plus1]float32
	if len(s.channels) == 1 {
		filter = s.channels[0].wienerFilter.filter
	} else {
		s.aggregateWienerFilters(&filter)
	}

	for c := range audio {
		fbs := &s.filterBankStates[c]
		for i := 0; i < fftSizeBy2Plus1; i++ {
			fbs.real[i] *= filter[i]
			fbs.imag[i] *= filter[i]
		}
	}

	for c := range audio {
		fbs := &s.filterBankStates[c]
		s.fft.inverse(&fbs.real, &fbs.imag, &fbs.extendedFrame)
	}

	for c := range audio {
		fbs := &s.filterBankStates[c]
		energyAfter := computeEnergyOfExtended(&fbs.extendedFrame)
		applyFilterBankWindow(&fbs.extendedFrame)
		s.gainAdjustments[c] = s.channels[c].wienerFilter.computeOverallScalingFactor(
			s.numAnalyzedFrames,
			s.channels[c].speechProb.priorSpeechProb,
			s.energiesBefore[c],
			energyAfter,
		)
	}

	gainAdjustment := s.gainAdjustments[0]
	for c := 1; c < len(s.channels); c++ {
		if s.gainAdjustments[c] < gainAdjustment {
			gainAdjustment = s.gainAdjustments[c]
		}
	}
	for c := range audio {
		fbs := &s.filterBankStates[c]
		for i := 0; i < fftSize; i++ {
			fbs.extendedFrame[i] *= gainAdjustment
		}
	}

	for c := range audio {
		fbs := &s.filterBankStates[c]
		overlapAndAdd(&fbs.extendedFrame, &s.channels[c].processSynthesisMemory, audio[c])
	}

	// Clip to [-32768, 32767] (legacy int16 range — upstream still enforces it).
	for c := range audio {
		for i, v := range audio[c] {
			if v > 32767 {
				audio[c][i] = 32767
			} else if v < -32768 {
				audio[c][i] = -32768
			}
		}
	}
	return nil
}

func (s *Suppressor) aggregateWienerFilters(filter *[fftSizeBy2Plus1]float32) {
	*filter = s.channels[0].wienerFilter.filter
	for c := 1; c < len(s.channels); c++ {
		f := &s.channels[c].wienerFilter.filter
		for k := 0; k < fftSizeBy2Plus1; k++ {
			if f[k] < filter[k] {
				filter[k] = f[k]
			}
		}
	}
}

func formExtendedFrame(frame []float32, oldData *[overlapSize]float32, extended *[fftSize]float32) {
	copy(extended[:overlapSize], oldData[:])
	copy(extended[overlapSize:], frame)
	copy(oldData[:], extended[fftSize-overlapSize:])
}

func overlapAndAdd(extended *[fftSize]float32, overlapMemory *[overlapSize]float32, outputFrame []float32) {
	for i := 0; i < overlapSize; i++ {
		outputFrame[i] = overlapMemory[i] + extended[i]
	}
	copy(outputFrame[overlapSize:nsFrameSize], extended[overlapSize:nsFrameSize])
	copy(overlapMemory[:], extended[nsFrameSize:])
}

func computeEnergyOfExtended(x *[fftSize]float32) float32 {
	var e float32
	for _, v := range x {
		e += v * v
	}
	return e
}

func computeEnergyFrameAndMemory(frame []float32, oldData *[overlapSize]float32) float32 {
	var e float32
	for _, v := range oldData {
		e += v * v
	}
	for _, v := range frame {
		e += v * v
	}
	return e
}

func computeMagnitudeSpectrum(real, imag, signalSpectrum *[fftSizeBy2Plus1]float32) {
	signalSpectrum[0] = abs32(real[0]) + 1
	signalSpectrum[fftSizeBy2Plus1-1] = abs32(real[fftSizeBy2Plus1-1]) + 1
	for i := 1; i < fftSizeBy2Plus1-1; i++ {
		signalSpectrum[i] = sqrtFastApprox(real[i]*real[i]+imag[i]*imag[i]) + 1
	}
}

func computeSnr(
	filter, prevSignalSpectrum, signalSpectrum, prevNoiseSpectrum, noiseSpectrum, priorSnr, postSnr *[fftSizeBy2Plus1]float32,
) {
	for i := 0; i < fftSizeBy2Plus1; i++ {
		prevEstimate := prevSignalSpectrum[i] / (prevNoiseSpectrum[i] + 0.0001) * filter[i]
		if signalSpectrum[i] > noiseSpectrum[i] {
			postSnr[i] = signalSpectrum[i]/(noiseSpectrum[i]+0.0001) - 1.0
		} else {
			postSnr[i] = 0
		}
		priorSnr[i] = 0.98*prevEstimate + (1.0-0.98)*postSnr[i]
	}
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
