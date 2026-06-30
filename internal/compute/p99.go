package compute

import (
	"fmt"
	"math"
	"sort"
)

// ComputeP99 calculates the 99th percentile of the given values.
// Values should be in consistent units (millicores for CPU, MiB for memory).
// Returns an error if the slice is empty.
func ComputeP99(values []float64) (float64, error) {
	if len(values) == 0 {
		return 0, fmt.Errorf("cannot compute p99 of empty dataset")
	}

	if len(values) == 1 {
		return values[0], nil
	}

	// step 1 — sort a copy so we don't modify the original slice
	var sorted []float64
	sorted = make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	// step 2 — calculate the index at the 99th percentile
	// ceil(0.99 × n) - 1 always stays within bounds since 0.99 × n < n
	var index int
	index = int(math.Ceil(0.99*float64(len(sorted)))) - 1

	return sorted[index], nil
}
