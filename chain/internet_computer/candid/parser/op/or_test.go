package op_test

import (
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser/op"
)

var OrTestCases = []OrTestCase{
	{"a", op.Or{'a'}},
	{"a", op.Or{'a', 'b'}},
	{"a", op.Or{'b', 'a'}},
	{"a", op.Or{"ab", 'a'}},
	{"ab", op.Or{"ab", 'a'}},
}

func TestOr(t *testing.T) {
	t.Run("Match", func(t *testing.T) {
		for _, test := range OrTestCases {
			p, err := parser.New([]rune(test.input))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.MatchEOF(test.consumer); err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("Parse", func(t *testing.T) {
		for _, test := range OrTestCases {
			p, err := parser.New([]rune(test.input))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.ParseEOF(test.consumer); err != nil {
				t.Fatal(err)
			}
		}
	})
}

type OrTestCase struct {
	input    string
	consumer op.Or
}
