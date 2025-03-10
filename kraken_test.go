package dca

import (
	"testing"
)

func TestTryParseFloat64OrZero(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"123.456", 123.456},
		{"0", 0.0},
		{"-987.654", -987.654},
		{"invalid", 0.0}, // Should return 0.0 for invalid input
		{"", 0.0},        // Should return 0.0 for empty string
		{"1e10", 1e10},   // Scientific notation
		{"NaN", 0.0},     // Should return 0.0 for NaN input
	}

	for i, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := tryParseFloat64OrZero(tc.input); got != tc.want {
				t.Errorf("(%d) tryParseFloat64OrZero(%q) want %v got %v", i, tc.input, tc.want, got)
			}
		})
	}
}
