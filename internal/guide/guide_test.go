package guide

import (
	"math"
	"slices"
	"testing"
)

func TestCrackSeconds(t *testing.T) {
	tests := []struct {
		bits, rate, want float64
	}{
		{1, 1, 1},
		{40, 2e10, math.Exp2(39) / 2e10}, // ~27 s
		{11, 1024, 1},
	}
	for _, tt := range tests {
		if got := CrackSeconds(tt.bits, tt.rate); math.Abs(got-tt.want) > 1e-9*tt.want {
			t.Errorf("CrackSeconds(%v, %v) = %v, want %v", tt.bits, tt.rate, got, tt.want)
		}
	}
	if got := FormatDuration(CrackSeconds(40, 2e10)); got != "27 s" {
		t.Errorf("40 bits @ 2e10/s = %q, want 27 s", got)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		s    float64
		want string
	}{
		{0.5, "instant"},
		{59, "59 s"},
		{60, "1 min"},
		{3600, "1 h"},
		{86400 * 2, "2 days"},
		{year * 10, "10 yrs"},
		{year * 5e3, "5k yrs"},
		{year * 7e6, "7M yrs"},
		{year * 2e9, "2.0G yrs"},
		{year * 1e11, "> universe"},
	}
	for _, tt := range tests {
		if got := FormatDuration(tt.s); got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestRows(t *testing.T) {
	if got := Rows(77.5); !slices.Contains(got, 77.5) || !slices.IsSorted(got) || len(got) != len(StandardBits)+1 {
		t.Errorf("Rows(77.5) = %v", got)
	}
	if got := Rows(80); len(got) != len(StandardBits) {
		t.Errorf("Rows(80) duplicated a row: %v", got)
	}
}

func TestFormatRate(t *testing.T) {
	tests := []struct {
		r    float64
		want string
	}{
		{1e3, "10³"},
		{2.5e10, "2.5·10¹⁰"},
		{2e1, "2·10¹"},
		{1e17, "10¹⁷"},
	}
	for _, tt := range tests {
		if got := FormatRate(tt.r); got != tt.want {
			t.Errorf("FormatRate(%v) = %q, want %q", tt.r, got, tt.want)
		}
	}
}
