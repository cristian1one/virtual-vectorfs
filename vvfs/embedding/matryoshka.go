package embedding

// AdjustToDims truncates or pads a vector to the target dimension.
// If target <= 0, returns the original slice.
func AdjustToDims(vec []float32, target int) []float32 {
	if target <= 0 {
		return vec
	}
	if len(vec) == target {
		return vec
	}
	if len(vec) > target {
		return vec[:target]
	}
	out := make([]float32, target)
	copy(out, vec)
	// leave tail zeros
	return out
}
