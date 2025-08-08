package op_test

import (
	"errors"
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser/op"
)

var AndTestCases = []AndTestCase{
	{"abc", op.And{'a', "bc"}},
	{"abc", op.And{op.Any{}, "bc"}},
	{"abc", op.And{op.Any{}, op.Any{}, 'c'}},
	{"abc", op.And{op.Or{'a', 'b'}, 'b', 'c'}},
	{"abc", op.And{op.Or{'b', 'a'}, 'b', 'c'}},
	{"abbbc", op.And{'a', op.OneOrMore{Value: op.And{op.Not{Value: op.And{'a', 'c'}}, 'b'}}, 'c'}},
}

func TestAnd(t *testing.T) {
	t.Run("Match", func(t *testing.T) {
		for _, test := range AndTestCases {
			p, err := parser.New([]rune(test.input))
			if err != nil {
				t.Fatal(err)
			}
			start := p.Reader.Cursor()
			c, err := p.Match(test.consumer)
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

func TestAnd_error(t *testing.T) {
	t.Run("Match", func(t *testing.T) {
		// If we try to match 'a' "bc" 'd' against "abc", we should get an error.
		// The returned cursor should be '-1', and the parser cursor should be at the start.
		p, err := parser.New([]rune("abc"))
		if err != nil {
			t.Fatal(err)
		}
		start := p.Reader.Cursor()
		c, err := p.Match(op.And{'a', "bc", 'd'})
		if err == nil {
			t.Fatal("expected error")
		}
		if c.Character() != -1 { // EOF
			t.Fatalf("expected cursor to be at '-1', got %c", c.Character())
		}
		if p.Reader.Cursor() != start {
			t.Fatal("expected cursor to be at start")
		}
	})
	t.Run("Parse", func(t *testing.T) {
		// If we try to parse 'a' "bc" against "abd", we should get an error.
		// The returned cursor should be 'b', and the parser cursor should be at the start.
		p, err := parser.New([]rune("abd"))
		if err != nil {
			t.Fatal(err)
		}
		start := p.Reader.Cursor()
		_, err = p.Parse(op.And{'a', "bc"})
		if err == nil {
			t.Fatal("expected error")
		}
		var stack *parser.ErrorStack
		errors.As(err, &stack)
		var match *parser.NoMatchError
		errors.As(stack.Errors[1], &match)
		if match.End.Character() != 'b' {
			t.Fatalf("expected cursor to be at 'b', got %c", match.End.Character())
		}
		if p.Reader.Cursor() != start {
			t.Fatal("expected cursor to be at start")
		}
	})
}

func TestAnd_flatten(t *testing.T) {
	p, err := parser.New([]rune("abcbc"))
	if err != nil {
		t.Fatal(err)
	}
	a := op.Capture{Name: "a", Value: 'a'}
	bc := op.Capture{Name: "bc", Value: "bc"}
	n, err := p.Parse(op.And{a, op.ZeroOrMore{Value: bc}})
	if err != nil {
		t.Fatal(err)
	}
	if len(n.Children()) != 3 {
		t.Fatalf("expected 3 children, got %d", len(n.Children()))
	}
}

type AndTestCase struct {
	input    string
	consumer op.And
}
