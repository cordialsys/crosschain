package op_test

import (
	"fmt"
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser/op"
)

var ZeroTestCases = []ZeroTestCase{
	{"aaa", op.ZeroOrMore{Value: 'a'}},
	{"aaaaa", op.ZeroOrMore{Value: 'a'}},
	{"abababaaa", op.ZeroOrMore{Value: op.Or{'a', 'b'}}},
}

func ExampleZeroOrMore_endsWith() {
	p, _ := parser.New([]rune("aa.a.a.a"))
	start := p.Reader.Cursor()
	c, err := p.Match(op.And{op.ZeroOrMore{Value: op.And{
		op.Or{'a', '.'},
		op.Peek{Value: op.Or{'a', '.'}},
	}}, 'a'})
	fmt.Println(string(p.Reader.GetInputRange(start, c)), err)
	// Output:
	// aa.a.a.a <nil>
}

func TestZero(t *testing.T) {
	t.Run("Match", func(t *testing.T) {
		for _, test := range ZeroTestCases {
			p, err := parser.New([]rune(test.input))
			if err != nil {
				t.Fatal(err)
			}
			start := p.Reader.Cursor()
			c, err := p.Match(op.And{test.consumer})
			if err != nil {
				t.Fatal(err)
			}
			out := string(p.Reader.GetInputRange(start, c))
			if out != test.input {
				t.Fatalf("expected %q, got %q", test.input, out)
			}
		}
	})
	t.Run("Parse", func(t *testing.T) {
		for _, test := range AndTestCases {
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

type ZeroTestCase struct {
	input    string
	consumer op.ZeroOrMore
}
