package candid_test

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/did"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/idl"
)

func ExampleEncodeValueString() {
	e, _ := candid.EncodeValueString("0")
	fmt.Printf("%x\n", e)
	// Output:
	// 4449444c00017c00
}

func ExampleEncodeValueString_blob() {
	e, _ := candid.EncodeValueString("(blob \"deadbeef\")")
	fmt.Printf("%x\n", e)
	// Output:
	// 4449444c016d7b0100086465616462656566
}

func ExampleParseDID() {
	raw, _ := os.ReadFile("testdata/counter.did")
	p, _ := did.ParseDID([]rune(string(raw)))
	fmt.Println(p)
	// Output:
	// service : {
	//   inc : () -> nat;
	// }
}

func TestDecodeValue(t *testing.T) {
	for _, test := range []struct {
		value   string
		encoded string
	}{
		{"(opt null)", "4449444c016e7f010000"},
		{"(opt 0)", "4449444c016e7c01000100"},

		{"(0 : nat)", "4449444c00017d00"},
		{"(0 : nat8)", "4449444c00017b00"},
		{"(0 : nat16)", "4449444c00017a0000"},
		{"(0 : nat32)", "4449444c00017900000000"},
		{"(0 : nat64)", "4449444c0001780000000000000000"},
		{"(0)", "4449444c00017c00"},
		{"(0 : int8)", "4449444c00017700"},
		{"(0 : int16)", "4449444c0001760000"},
		{"(0 : int32)", "4449444c00017500000000"},
		{"(0 : int64)", "4449444c0001740000000000000000"},

		{"(0 : float32)", "4449444c00017300000000"},
		{"(0 : float64)", "4449444c0001720000000000000000"},
		{"(1 : float64)", "4449444c000172000000000000f03f"},

		{"(true)", "4449444c00017e01"},
		{"(false)", "4449444c00017e00"},

		{"(null)", "4449444c00017f"},

		{"(\"\")", "4449444c00017100"},
		{"(\"quint\")", "4449444c000171057175696e74"},

		{"(record {})", "4449444c016c000100"},
		{"(record { 4895187 = 42; 5097222 = \"baz\" })", "4449444c016c02d3e3aa027c868eb7027101002a0362617a"},

		{"(variant { 24860 })", "4449444c016b019cc2017f010000"},
		{"(variant { 5048165 = \"oops...\" })", "4449444c016b01e58eb40271010000076f6f70732e2e2e"},

		{"(vec {})", "4449444c016d7f010000"},
		{"(vec { 0 })", "4449444c016d7c01000100"},

		{"(opt principal \"aaaaa-aa\")", "4449444c016e680100010100"},
	} {
		e, err := hex.DecodeString(test.encoded)
		if err != nil {
			t.Fatal(err)
		}
		d, err := candid.DecodeValueString(e)
		if err != nil {
			t.Fatal(err)
		}
		if d != test.value {
			t.Error(test, d)
		}
	}
}

func TestDecodeValues(t *testing.T) {
	for _, test := range []struct {
		value  string
		types  []idl.Type
		values []any
	}{
		{
			value:  "(0 : nat)",
			types:  []idl.Type{new((idl.NatType))},
			values: []any{new(big.Int)},
		},
	} {
		d, err := candid.DecodeValuesString(test.types, test.values)
		if err != nil {
			t.Fatal(err)
		}
		if d != test.value {
			t.Error(test, d)
		}
	}
}

func TestEncodeValue(t *testing.T) {
	for _, test := range []struct {
		value   string
		encoded string
	}{
		{"opt null", "4449444c016e7f010000"},
		{"opt 0", "4449444c016e7c01000100"},

		{"0", "4449444c00017c00"},
		{"(0)", "4449444c00017c00"},
		{"(0 : nat)", "4449444c00017d00"},
		{"(0 : nat8)", "4449444c00017b00"},
		{"(0 : nat16)", "4449444c00017a0000"},
		{"(0 : nat32)", "4449444c00017900000000"},
		{"(0 : nat64)", "4449444c0001780000000000000000"},
		{"(0 : int)", "4449444c00017c00"},
		{"(0 : int8)", "4449444c00017700"},
		{"(0 : int16)", "4449444c0001760000"},
		{"(0 : int32)", "4449444c00017500000000"},
		{"(0 : int64)", "4449444c0001740000000000000000"},

		{"0.0", "4449444c0001720000000000000000"},
		{"(0 : float32)", "4449444c00017300000000"},
		{"(0.0 : float32)", "4449444c00017300000000"},
		{"(0 : float64)", "4449444c0001720000000000000000"},
		{"(0.0 : float64)", "4449444c0001720000000000000000"},

		{"true", "4449444c00017e01"},
		{"(false : bool)", "4449444c00017e00"},

		{"(null)", "4449444c00017f"},

		{"\"\"", "4449444c00017100"},
		{"\"quint\"", "4449444c000171057175696e74"},

		{"record {}", "4449444c016c000100"},
		{"record {foo = \"baz\"; bar = 42}", "4449444c016c02d3e3aa027c868eb7027101002a0362617a"},

		{"variant { ok }", "4449444c016b019cc2017f010000"},
		{"variant { err = \"oops...\" }", "4449444c016b01e58eb40271010000076f6f70732e2e2e"},

		{"vec {}", "4449444c016d7f010000"},
		{"vec { 0; }", "4449444c016d7c01000100"},
	} {
		e, err := candid.EncodeValueString(test.value)
		if err != nil {
			t.Fatal(err)
		}
		if e := fmt.Sprintf("%x", e); e != test.encoded {
			t.Error(test, e)
		}
	}
}

func TestParseDID(t *testing.T) {
	raw, _ := os.ReadFile("internal/candid/testdata/ic.did")
	if _, err := did.ParseDID([]rune(string(raw))); err != nil {
		t.Error(err)
	}
}

func TestRecordSubtyping(t *testing.T) {
	type T struct{ X idl.Nat }
	type T_ struct {
		X idl.Nat
		Y *idl.Nat
	}
	ONE := idl.NewNat(uint(1))
	t.Run("missing field", func(t *testing.T) {
		encoded, err := candid.Marshal([]any{T_{X: ONE, Y: &ONE}})
		if err != nil {
			t.Fatal(err)
		}
		var value T
		if err := candid.Unmarshal(encoded, []any{&value}); err != nil {
			t.Fatal(err)
		}
		if value.X.BigInt().Int64() != 1 {
			t.Error(value.X)
		}
	})
	t.Run("extra field", func(t *testing.T) {
		encoded, err := candid.Marshal([]any{T{X: ONE}})
		if err != nil {
			t.Fatal(err)
		}
		var value T_
		if err := candid.Unmarshal(encoded, []any{&value}); err != nil {
			t.Fatal(err)
		}
		if value.X.BigInt().Int64() != 1 {
			t.Error(value.X)
		}
		if value.Y != nil {
			t.Error(value.Y)
		}
	})
}
