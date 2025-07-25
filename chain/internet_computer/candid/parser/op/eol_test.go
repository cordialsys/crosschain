package op_test

import (
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser/op"
)

var EOLTestCases = []EOLTestCase{
	{"\n"},
	{"\r"},
	{"\n\r"},
}

func TestEOL(t *testing.T) {
	eol := op.And{op.EndOfLine{}} // op.EOL{}
	t.Run("Match", func(t *testing.T) {
		for _, test := range EOLTestCases {
			p, err := parser.New([]rune(test.input))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.MatchEOF(eol); err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("Parse", func(t *testing.T) {
		for _, test := range EOLTestCases {
			p, err := parser.New([]rune(test.input))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.Parse(eol); err != nil {
				t.Fatal(err)
			}
		}
	})
}

type EOLTestCase struct {
	input string
}
