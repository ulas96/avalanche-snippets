package main

import "testing"

func TestParseAmount(t *testing.T) {
	for _, c := range []struct {
		in    string
		nAVAX uint64
		ok    bool
	}{
		{"0.05", 50_000_000, true},
		{"1", 1_000_000_000, true},
		{"0.000000001", 1, true},
		{"0.1", 100_000_000, true}, // float rounding must not give 99,999,999
		{"0.0000000001", 0, false}, // rounds to 0 nAVAX
		{"0", 0, false},
		{"-1", 0, false},
		{"", 0, false},
		{"abc", 0, false},
		{"NaN", 0, false},
		{"Inf", 0, false},
	} {
		_, got, ok := parseAmount(c.in)
		if got != c.nAVAX || ok != c.ok {
			t.Errorf("parseAmount(%q) = %d, %v; want %d, %v", c.in, got, ok, c.nAVAX, c.ok)
		}
	}
}
