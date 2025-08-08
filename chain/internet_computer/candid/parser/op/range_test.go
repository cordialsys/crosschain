package op_test

import (
	"testing"
	"unicode/utf8"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser/op"
)

func Test0xFFFD(t *testing.T) {
	for _, r := range []rune{utf8.RuneError, 0x10FFFF} {
		p, err := parser.New([]rune{r})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.ParseEOF(op.RuneRange{Min: 0x7D, Max: 0x10FFFF}); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ParseEOF(op.ZeroOrMore{Value: op.RuneRange{Min: 0x7D, Max: 0x10FFFF}}); err != nil {
			t.Fatal(err)
		}
	}
}
