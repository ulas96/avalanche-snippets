package main

import (
	"encoding/hex"
	"testing"

	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
)

func TestParseKey(t *testing.T) {
	want, err := secp256k1.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	h := hex.EncodeToString(want.Bytes())

	for _, in := range []string{h, "0x" + h, "  " + h + "\n", want.String()} {
		got, err := parseKey(in)
		if err != nil {
			t.Errorf("parseKey(%q): %s", in, err)
			continue
		}
		if got.Address() != want.Address() {
			t.Errorf("parseKey(%q) gave a different key", in)
		}
	}

	for _, in := range []string{"", "   ", "zz", "0xabcd", "PrivateKey-notbase58"} {
		if _, err := parseKey(in); err == nil {
			t.Errorf("parseKey(%q) should fail", in)
		}
	}
}

func TestPow10(t *testing.T) {
	for n, want := range map[int]uint64{0: 1, 1: 10, 2: 100, 9: 1_000_000_000, 19: 10_000_000_000_000_000_000} {
		if got := pow10(n); got != want {
			t.Errorf("pow10(%d) = %d, want %d", n, got, want)
		}
	}
}

func TestFormatAVAX(t *testing.T) {
	for in, want := range map[uint64]string{0: "0", 10_000_000: "0.01", 1_000_000_000: "1", 1_500_000_000: "1.5", 1: "0.000000001"} {
		if got := formatAVAX(in); got != want {
			t.Errorf("formatAVAX(%d) = %q, want %q", in, got, want)
		}
	}
}
