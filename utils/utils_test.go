package utils

import (
	"testing"
)

func TestMinMax(t *testing.T) {

	tester := func(arr []float64, exMin, exMax float64, exError bool) {
		min, max, err := MinMax(arr)
		if min != exMin {
			t.Error("Expected min=", exMin, "got", min)
		}
		if max != exMax {
			t.Error("Expected max=", exMax, "got", max)
		}
		if err != nil && !exError {
			t.Error("Expected no error, got error:", err)
		} else if err == nil && exError {
			t.Error("Expected an error but there wasn't one")
		}
	}

	tester([]float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8}, 1.1, 8.8, false)
	tester([]float64{1.1, 1.1, 1.1, 1.1, 1.1, 1.1}, 1.1, 1.1, false)
	tester([]float64{}, 0, 0, true)

}
