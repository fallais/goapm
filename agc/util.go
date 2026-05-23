package agc

import "math"

// floatS16ToDbfs returns 20·log10(|x|/32768).
func floatS16ToDbfs(x float32) float32 {
	if x < 0 {
		x = -x
	}
	if x < 1e-7 {
		return kMinLevelDbfs
	}
	return float32(20 * math.Log10(float64(x)/kMaxAbsFloatS16Value))
}

func safeClamp(x, lo, hi float32) float32 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
