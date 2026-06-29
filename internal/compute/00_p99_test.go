package compute

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeP99_Empty(t *testing.T) {
	_, err := ComputeP99([]float64{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty dataset")
}

func TestComputeP99_SingleValue(t *testing.T) {
	result, err := ComputeP99([]float64{120.5})
	assert.NoError(t, err)
	assert.Equal(t, 120.5, result)
}

func TestComputeP99_TwoValues(t *testing.T) {
	result, err := ComputeP99([]float64{100.0, 200.0})
	assert.NoError(t, err)
	// ceil(0.99 * 2) = ceil(1.98) = 2, index = 1 → 200.0
	assert.Equal(t, 200.0, result)
}

func TestComputeP99_IgnoresTopSpike(t *testing.T) {
	// 100 values: 99 normal values (100.0) and 1 huge spike (9999.0)
	values := make([]float64, 99)
	for i := range values {
		values[i] = 100.0
	}
	values = append(values, 9999.0) // spike at the top

	result, err := ComputeP99(values)
	assert.NoError(t, err)
	// p99 of 100 values — index = ceil(0.99*100)-1 = 99-1 = 98 → 100.0
	// the spike at position 99 is ignored
	assert.Equal(t, 100.0, result)
}

func TestComputeP99_ReturnsCorrectPosition(t *testing.T) {
	// 10 values: 1,2,3,4,5,6,7,8,9,10
	values := []float64{5, 3, 8, 1, 9, 2, 7, 4, 10, 6}

	result, err := ComputeP99(values)
	assert.NoError(t, err)
	// sorted: [1,2,3,4,5,6,7,8,9,10]
	// ceil(0.99 * 10) = ceil(9.9) = 10, index = 10-1 = 9 → value = 10
	assert.Equal(t, 10.0, result)
}

func TestComputeP99_DoesNotModifyOriginal(t *testing.T) {
	original := []float64{5.0, 3.0, 8.0, 1.0, 9.0}
	originalCopy := make([]float64, len(original))
	copy(originalCopy, original)

	ComputeP99(original)

	// original slice must be unchanged after computation
	assert.Equal(t, originalCopy, original)
}

func TestComputeP99_Typical7DayWorkload(t *testing.T) {
	// simulate 168 data points (7 days × 24 hours)
	// 167 normal values at 120.0 + 1 spike at 9999.0 (top 0.6%)
	values := make([]float64, 167)
	for i := range values {
		values[i] = 120.0
	}
	values = append(values, 9999.0) // 1 spike = top 0.6%

	result, err := ComputeP99(values)
	assert.NoError(t, err)
	// p99 index = ceil(0.99 * 168) - 1 = 166 → sorted[166] = 120.0
	// the spike at position 167 is beyond p99 — correctly ignored
	assert.Equal(t, 120.0, result)
}
