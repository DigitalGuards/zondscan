package hexutil

import (
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"
)

// The ParseInt64 table is seeded from the db package's logIndexSortKey
// tests (db/db_test.go), adapted to this package's parse-and-report
// contract: where the sort key maps empty/malformed input to -1/0
// sentinels, ParseInt64 surfaces an error and leaves the sentinel
// decision to the caller.

func TestParseInt64(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    int64
		wantErr bool
		// rangeErr asserts the error is exactly ErrRange (the
		// fits-uint64-but-not-int64 clamp signal callers like
		// db.HexToInt key their MaxInt64 behaviour on).
		rangeErr bool
	}{
		{name: "empty is an error", in: "", wantErr: true},
		{name: "zero hex", in: "0x0", want: 0},
		{name: "single hex digit", in: "0x9", want: 9},
		{name: "two digit hex", in: "0xa", want: 10},
		{name: "width crossover", in: "0x10", want: 16},
		// Historical helpers only trimmed the lowercase prefix; "0X"
		// left the 'X' in the digit stream and failed the parse. That
		// behaviour is load-bearing for db.HexToInt's zero fallback.
		{name: "uppercase prefix rejected", in: "0X1f", wantErr: true},
		{name: "mixed case body", in: "0xAb", want: 171},
		{name: "malformed is an error", in: "not-hex", wantErr: true},
		// Bare hex (no 0x prefix) is accepted; the syncer always writes
		// the 0x form but the defensive behaviour is locked in here so a
		// caller passing "1234" is not surprised.
		{name: "naked hex parsed as hex too", in: "1234", want: 0x1234},
		{name: "int64 max sanity", in: "0x7fffffffffffffff", want: 0x7fffffffffffffff},
		// Fits uint64, overflows int64: saturates to MaxInt64 and
		// signals ErrRange so callers choose clamp vs fail.
		{name: "int64 overflow saturates with ErrRange", in: "0x8000000000000000", want: math.MaxInt64, wantErr: true, rangeErr: true},
		{name: "uint64 max saturates with ErrRange", in: "0xffffffffffffffff", want: math.MaxInt64, wantErr: true, rangeErr: true},
		// Does not fit uint64 at all: plain strconv range error, zero
		// value (matches the historical garbage-yields-zero contract).
		{name: "uint64 overflow is a plain error", in: "0xffffffffffffffffff", want: 0, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseInt64(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseInt64(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			}
			if tc.rangeErr != errors.Is(err, ErrRange) {
				t.Errorf("ParseInt64(%q) errors.Is(err, ErrRange) = %v, want %v (err = %v)", tc.in, !tc.rangeErr, tc.rangeErr, err)
			}
			if got != tc.want {
				t.Errorf("ParseInt64(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseBig(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string // decimal form; "" means expect an error
		wantErr bool
	}{
		{name: "empty is an error", in: "", wantErr: true},
		{name: "bare prefix is an error", in: "0x", wantErr: true},
		{name: "zero hex", in: "0x0", want: "0"},
		{name: "two digit hex", in: "0xa", want: "10"},
		{name: "mixed case body", in: "0xAb", want: "171"},
		{name: "malformed is an error", in: "not-hex", wantErr: true},
		// Partial parses must fail, never truncate: "12zz" is not 0x12.
		{name: "trailing garbage is an error", in: "0x12zz", wantErr: true},
		{name: "naked hex parsed as hex too", in: "1234", want: "4660"},
		{name: "int64 max sanity", in: "0x7fffffffffffffff", want: "9223372036854775807"},
		// The whole reason gas math uses big.Int: values past 64 bits
		// parse losslessly.
		{name: "past uint64", in: "0xffffffffffffffffff", want: "4722366482869645213695"},
		{name: "uint256 max", in: "0x" + strings.Repeat("f", 64), want: func() string {
			m := new(big.Int).Lsh(big.NewInt(1), 256)
			return m.Sub(m, big.NewInt(1)).String()
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseBig(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseBig(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			}
			if tc.wantErr {
				if got != nil {
					t.Errorf("ParseBig(%q) = %v on error, want nil", tc.in, got)
				}
				return
			}
			if got.String() != tc.want {
				t.Errorf("ParseBig(%q) = %s, want %s", tc.in, got.String(), tc.want)
			}
		})
	}
}
