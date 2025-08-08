package leb128_test

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/leb128"
)

func newInt(t *testing.T, str string) *big.Int {
	bi, ok := new(big.Int).SetString(str, 10)
	if !ok {
		t.Fatal()
	}
	return bi
}

func TestUnsigned(t *testing.T) {
	for _, test := range []struct {
		Hex   string
		Value *big.Int
	}{
		{"00", big.NewInt(0)},
		{"07", big.NewInt(7)},
		{"7F", big.NewInt(127)},
		{"E58E26", big.NewInt(624485)},
		{"80897A", big.NewInt(2000000)},
		{"808098F4E9B5CA6A", big.NewInt(60000000000000000)},
		{"EF9BAF8589CF959A92DEB7DE8A929EABB424", newInt(t, "24197857200151252728969465429440056815")},
	} {
		t.Run(test.Hex, func(t *testing.T) {
			e := new(big.Int).Set(test.Value)
			bs, err := leb128.EncodeUnsigned(e)
			if err != nil {
				t.Fatal(err)
			}
			if h := fmt.Sprintf("%X", bs); h != test.Hex {
				t.Errorf("\n%50s\n%50s", h, test.Hex)
			}

			d := new(big.Int).Set(test.Value)
			r := bytes.NewReader(bs)
			bi, err := leb128.DecodeUnsigned(r)
			if err != nil {
				t.Fatal(err)
			}
			if bi.Cmp(d) != 0 {
				t.Errorf("%s, \n%s\n%s", test.Hex, d, bi)
			}
			if r.Len() != 0 {
				t.Error()
			}
		})
	}
}

func TestUnsignedMultiple(t *testing.T) {
	v := big.NewInt(127)
	b, err := leb128.EncodeUnsigned(v)
	if err != nil {
		t.Fatal(err)
	}
	var bs []byte
	for i := 0; i < 10; i++ {
		bs = append(bs, b...)
	}
	r := bytes.NewReader(bs)
	for i := 0; i < 10; i++ {
		bi, err := leb128.DecodeUnsigned(r)
		if err != nil {
			t.Error(err)
		}
		if bi.Cmp(v) != 0 {
			t.Error(bi)
		}
	}
	if r.Len() != 0 {
		raw, _ := io.ReadAll(r)
		t.Fatalf("%x", raw)
	}
}

func TestSigned(t *testing.T) {
	for _, test := range []struct {
		Hex   string
		Value *big.Int
	}{
		{"2A", big.NewInt(42)},
		{"7F", big.NewInt(-1)},
		{"C0BB78", big.NewInt(-123456)},
		{"8089FA00", big.NewInt(2000000)},
		{"808098F4E9B5CAEA00", big.NewInt(60000000000000000)},
		{"EF9BAF8589CF959A92DEB7DE8A929EABB424", newInt(t, "24197857200151252728969465429440056815")},
		{"91E4D0FAF6B0EAE5EDA1C8A1F5EDE1D4CB5B", newInt(t, "-24197857200151252728969465429440056815")},
	} {
		t.Run(test.Hex, func(t *testing.T) {
			e := new(big.Int).Set(test.Value)
			bs, err := leb128.EncodeSigned(e)
			if err != nil {
				t.Fatal(err)
			}
			if h := fmt.Sprintf("%X", bs); h != test.Hex {
				t.Errorf("\n%50s\n%50s", h, test.Hex)
			}

			d := new(big.Int).Set(test.Value)
			r := bytes.NewReader(bs)
			bi, err := leb128.DecodeSigned(r)
			if err != nil {
				t.Fatal(err)
			}
			if bi.Cmp(d) != 0 {
				t.Errorf("%s, \n%s\n%s", test.Hex, d, bi)
			}
			if r.Len() != 0 {
				t.Error()
			}
		})
	}
}

func TestSignedMultiple(t *testing.T) {
	v := big.NewInt(-1)
	b, err := leb128.EncodeSigned(v)
	if err != nil {
		t.Fatal(err)
	}
	var bs []byte
	for i := 0; i < 10; i++ {
		bs = append(bs, b...)
	}
	r := bytes.NewReader(bs)
	for i := 0; i < 10; i++ {
		bi, err := leb128.DecodeSigned(r)
		if err != nil {
			t.Error(err)
		}
		if bi.Cmp(v) != 0 {
			t.Error(bi)
		}
	}
	if r.Len() != 0 {
		raw, _ := io.ReadAll(r)
		t.Fatalf("%x", raw)
	}
}

func TestSignedTooShort(t *testing.T) {
	raw, _ := leb128.EncodeSigned(big.NewInt(128))
	// [x80, x01]
	b, _ := leb128.DecodeSigned(bytes.NewReader(raw))
	if b.Cmp(big.NewInt(128)) != 0 {
		t.Fatal(b)
	}
	// [x80]
	if _, err := leb128.DecodeSigned(bytes.NewReader(raw[:len(raw)-2])); err == nil {
		t.Error()
	}
}
