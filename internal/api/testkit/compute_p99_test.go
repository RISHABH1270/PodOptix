package testkit

import (
	"testing"

	"github.com/RISHABH1270/PodOptix/internal/compute"
	"github.com/stretchr/testify/assert"
)

func TestComputeP99(t *testing.T) {
	t.Run("empty dataset returns error", func(t *testing.T) {
		track(t)
		_, err := compute.ComputeP99([]float64{})
		assert.ErrorContains(t, err, "empty dataset")
	})

	t.Run("single value returns that value", func(t *testing.T) {
		track(t)
		result, err := compute.ComputeP99([]float64{120.5})
		assert.NoError(t, err)
		assert.Equal(t, 120.5, result)
	})

	t.Run("two values returns highest", func(t *testing.T) {
		track(t)
		result, err := compute.ComputeP99([]float64{100.0, 200.0})
		assert.NoError(t, err)
		assert.Equal(t, 200.0, result)
	})

	t.Run("correct index position", func(t *testing.T) {
		track(t)
		result, err := compute.ComputeP99([]float64{5, 3, 8, 1, 9, 2, 7, 4, 10, 6})
		assert.NoError(t, err)
		assert.Equal(t, 10.0, result)
	})

	t.Run("top 1% spike ignored", func(t *testing.T) {
		track(t)
		values := make([]float64, 99)
		for i := range values {
			values[i] = 100.0
		}
		values = append(values, 9999.0)
		result, err := compute.ComputeP99(values)
		assert.NoError(t, err)
		assert.Equal(t, 100.0, result)
	})

	t.Run("original slice not modified", func(t *testing.T) {
		track(t)
		original := []float64{5.0, 3.0, 8.0, 1.0, 9.0}
		snapshot := make([]float64, len(original))
		copy(snapshot, original)
		compute.ComputeP99(original)
		assert.Equal(t, snapshot, original)
	})

	t.Run("typical 7d workload — spike beyond p99 ignored", func(t *testing.T) {
		track(t)
		values := make([]float64, 167)
		for i := range values {
			values[i] = 120.0
		}
		values = append(values, 9999.0)
		result, err := compute.ComputeP99(values)
		assert.NoError(t, err)
		assert.Equal(t, 120.0, result)
	})
}
