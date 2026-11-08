package model

import "testing"

func TestCollectorNumberOrderingPreservesNumericRunsAndSuffixes(t *testing.T) {
	for _, pair := range [][2]string{
		{"2", "10"}, {"2", "2a"}, {"2a", "2b"}, {"", "1"},
		{"999999999999999999999999", "1000000000000000000000000"},
		{"1-2", "1-10"}, {"10", "10★"},
	} {
		if CompareCollectorNumbers(pair[0], pair[1]) >= 0 || CompareCollectorNumbers(pair[1], pair[0]) <= 0 {
			t.Fatalf("incorrect collector-number order: %v", pair)
		}
	}
	if CompareCollectorNumbers("02", "2") != 0 {
		t.Fatal("equal numeric values must preserve the printing-UUID tie break")
	}
}
