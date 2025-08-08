package op_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser/op"
)

func ExampleAny() {
	p, _ := parser.New([]rune("abc"))
	_, err := p.Match(op.Repeat{Min: 4, Max: 4, Value: op.Any{}})
	fmt.Println(err)
	// Output:
	// [1:4/1:4] '�' | no match: .
	// abc
	// ---^
}

func TestAny_Match(t *testing.T) {
	input := "abc"
	p, err := parser.New([]rune(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range input {
		if character := p.Reader.Cursor().Character(); character != c {
			t.Fatalf("expected %c, got %c", c, character)
		}
		if _, err := p.Match(op.Any{}); err != nil {
			t.Fatal(err)
		}
	}
	// No more characters to match.
	if _, err := p.Match(op.Any{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestAny_error(t *testing.T) {
	// If we try to match/parse '.' against "", we should get an error.
	// The returned cursor should be '-1', and the parser cursor should be at the start.
	t.Run("Match", func(t *testing.T) {
		p, err := parser.New([]rune("a"))
		if err != nil {
			t.Fatal(err)
		}
		start := p.Reader.Cursor()
		c, err := p.Match(op.And{'a', op.Any{}})
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
		p, err := parser.New([]rune("a"))
		if err != nil {
			t.Fatal(err)
		}
		start := p.Reader.Cursor()
		_, err = p.Parse(op.And{'a', op.Any{}})
		if err == nil {
			t.Fatal("expected error")
		}
		var stack *parser.ErrorStack
		errors.As(err, &stack)
		var match *parser.NoMatchError
		errors.As(stack.Errors[0], &match)
		if match.End.Character() != -1 { // EOF
			t.Fatalf("expected cursor to be at '-1', got %c", match.End.Character())
		}
		if p.Reader.Cursor() != start {
			t.Fatal("expected cursor to be at start")
		}
	})
}
