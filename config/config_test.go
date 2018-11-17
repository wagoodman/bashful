package config

import (
	"github.com/alecthomas/repr"
	"github.com/wagoodman/bashful/utils"
	"testing"
)

func TestRemoveOneValue(t *testing.T) {
	eq := func(a, b []float64) bool {

		if a == nil && b == nil {
			return true
		}

		if a == nil || b == nil {
			return false
		}

		if len(a) != len(b) {
			return false
		}

		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}

		return true
	}

	tester := func(arr []float64, value float64, exArr []float64) {
		testArr := utils.RemoveOneValue(arr, value)
		if !eq(testArr, exArr) {
			t.Error("Expected", repr.String(exArr), "got", repr.String(testArr))
		}
	}

	tester([]float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8}, 1.1, []float64{2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8})
	tester([]float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8}, 3.14159, []float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8})
	tester([]float64{1.1, 1.1, 1.1, 1.1, 1.1, 1.1}, 1.1, []float64{1.1, 1.1, 1.1, 1.1, 1.1})
	tester([]float64{}, 3.14159, []float64{})

}
