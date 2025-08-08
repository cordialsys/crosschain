package op_test

import (
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser/op"
)

func TestCapture(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		p, err := parser.New([]rune("abc"))
		if err != nil {
			t.Fatal(err)
		}
		n, err := p.Parse(op.Capture{Name: "test", Value: "abc"})
		if err != nil {
			t.Fatal(err)
		}
		if n.Name != "test" {
			t.Fatalf("expected name to be 'test', got '%s'", n.Name)
		}
		if v := n.Value(); v != "abc" {
			t.Fatalf("expected value to be 'abc', got '%s'", v)
		}
	})
	t.Run("Nested Single", func(t *testing.T) {
		t.Run("Anonymous Capture", func(t *testing.T) {
			p, err := parser.New([]rune("abc"))
			if err != nil {
				t.Fatal(err)
			}
			n, err := p.Parse(op.Capture{Name: "test", Value: op.Capture{Value: "abc"}})
			if err != nil {
				t.Fatal(err)
			}
			if n.Name != "test" {
				t.Fatalf("expected name to be 'test', got '%s'", n.Name)
			}
			if v := n.Value(); v != "abc" {
				t.Fatalf("expected value to be 'abc', got '%s'", v)
			}
		})
		t.Run("Named Capture", func(t *testing.T) {
			p, err := parser.New([]rune("abc"))
			if err != nil {
				t.Fatal(err)
			}
			n, err := p.Parse(op.Capture{Name: "test", Value: op.Capture{Name: "child", Value: "abc"}})
			if err != nil {
				t.Fatal(err)
			}
			if n.Name != "test" {
				t.Fatalf("expected name to be 'test', got '%s'", n.Name)
			}
			if l := len(n.Children()); l != 1 {
				t.Fatalf("expected 1 child, got %d", l)
			}
			if n.Children()[0].Name != "child" {
				t.Fatalf("expected child name to be 'child', got '%s'", n.Children()[0].Name)
			}
			if v := n.Children()[0].Value(); v != "abc" {
				t.Fatalf("expected value to be 'abc', got '%s'", v)
			}
		})
	})
	t.Run("Nested Multiple Capture", func(t *testing.T) {
		p, err := parser.New([]rune("abc"))
		if err != nil {
			t.Fatal(err)
		}
		n, err := p.Parse(op.Capture{Name: "test", Value: op.And{op.Capture{Name: "ab", Value: "ab"}, op.Capture{Name: "c", Value: 'c'}}})
		if err != nil {
			t.Fatal(err)
		}
		if n.Name != "test" {
			t.Fatalf("expected name to be 'test', got '%s'", n.Name)
		}
		if l := len(n.Children()); l != 2 {
			t.Fatalf("expected 2 child, got %d", l)
		}
		var str string
		for _, c := range n.Children() {
			str += c.Value()
		}
		if str != "abc" {
			t.Fatalf("expected value to be 'abc', got '%s'", str)
		}
	})
	t.Run("Nested 2 Levels", func(t *testing.T) {
		t.Run("Anonymous Capture", func(t *testing.T) {
			p, err := parser.New([]rune("abc"))
			if err != nil {
				t.Fatal(err)
			}
			n, err := p.Parse(op.Capture{Name: "test", Value: op.And{'a', op.Capture{Name: "bc", Value: "bc"}}})
			if err != nil {
				t.Fatal(err)
			}
			if n.Name != "test" {
				t.Fatalf("expected name to be 'test', got '%s'", n.Name)
			}
			if v := n.Value(); v != "" {
				t.Fatalf("expected value to be 'bc', got '%s'", v)
			}
			if l := len(n.Children()); l != 1 {
				t.Fatalf("expected 1 child, got %d", l)
			}
			if n.Children()[0].Name != "bc" {
				t.Fatalf("expected child name to be 'bc', got '%s'", n.Children()[0].Name)
			}
			if v := n.Children()[0].Value(); v != "bc" {
				t.Fatalf("expected value to be 'bc', got '%s'", v)
			}
		})
		t.Run("Anonymous Capture", func(t *testing.T) {
			p, err := parser.New([]rune("abc"))
			if err != nil {
				t.Fatal(err)
			}
			n, err := p.Parse(op.Capture{Name: "test", Value: op.And{'a', op.Capture{Name: "child", Value: "bc"}}})
			if err != nil {
				t.Fatal(err)
			}
			if n.Name != "test" {
				t.Fatalf("expected name to be 'test', got '%s'", n.Name)
			}
			if l := len(n.Children()); l != 1 {
				t.Fatalf("expected 1 child, got %d", l)
			}
			if n.Children()[0].Name != "child" {
				t.Fatalf("expected child name to be 'child', got '%s'", n.Children()[0].Name)
			}
			if v := n.Children()[0].Value(); v != "bc" {
				t.Fatalf("expected value to be 'bc', got '%s'", v)
			}
		})
	})
	t.Run("Nested 3 Levels", func(t *testing.T) {
		p, err := parser.New([]rune("abc"))
		if err != nil {
			t.Fatal(err)
		}
		n, err := p.Parse(op.Capture{Name: "test", Value: op.And{'a', op.And{'b', op.Capture{Name: "child", Value: 'c'}}}})
		if err != nil {
			t.Fatal(err)
		}
		if n.Name != "test" {
			t.Fatalf("expected name to be 'test', got '%s'", n.Name)
		}
		if l := len(n.Children()); l != 1 {
			t.Fatalf("expected 1 child, got %d", l)
		}
		if n.Children()[0].Name != "child" {
			t.Fatalf("expected child name to be 'child', got '%s'", n.Children()[0].Name)
		}
		if v := n.Children()[0].Value(); v != "c" {
			t.Fatalf("expected value to be 'bc', got '%s'", v)
		}
	})
}

func TestCapture_Parse_children(t *testing.T) {
	input := "abc"
	for _, test := range []struct {
		op     parser.Operator
		amount int
	}{
		{op.Capture{Value: input}, 0},
		{op.Capture{Name: "child", Value: input}, 1},
		{op.And{'a', op.Capture{Name: "child", Value: "bc"}}, 1},
		{op.And{'a', op.Capture{Name: "child", Value: op.ZeroOrMore{Value: op.Or{'a', 'b', 'c'}}}}, 1},
	} {
		p, err := parser.New([]rune(input))
		if err != nil {
			t.Fatal(err)
		}
		n, err := p.ParseEOF(op.Capture{Name: "test", Value: test.op})
		if err != nil {
			t.Fatal(err)
		}
		if l := len(n.Children()); l != test.amount {
			t.Errorf("expected %d children, got %d", test.amount, l)
		}
	}
}
