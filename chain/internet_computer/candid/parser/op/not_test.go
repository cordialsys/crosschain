package op_test

import (
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser/op"
)

var NotTestCases = []NotTestCase{
	{"a", op.Not{Value: "b"}},
	{"a", op.Not{Value: "ab"}},
}

func TestNot(t *testing.T) {
	t.Run("Match", func(t *testing.T) {
		for _, test := range NotTestCases {
			p, err := parser.New([]rune(test.input))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.MatchEOF(op.And{test.consumer, test.input}); err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("Parse", func(t *testing.T) {
		for _, test := range NotTestCases {
			p, err := parser.New([]rune(test.input))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.ParseEOF(op.And{test.consumer, test.input}); err != nil {
				t.Fatal(err)
			}
		}
	})
}

type NotTestCase struct {
	input    string
	consumer op.Not
}
