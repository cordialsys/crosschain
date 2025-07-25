package parser_test

import (
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser/op"
)

func TestParser_Parse_string(t *testing.T) {
	for _, test := range []string{
		`      " t e s t "      `,
		`" t e s t "" t e s t "`,
		`" t e s t " " t e s t "`,
		` " t e s t " " t e s t " `,
	} {
		p, err := parser.New([]rune(test))
		if err != nil {
			t.Fatal(err)
		}
		p.SetIgnoreList([]any{' '})
		n, err := p.ParseEOF(op.OneOrMore{
			Value: op.Capture{
				Name:  "String",
				Value: op.And{'"', op.ZeroOrMore{Value: op.AnyBut{Value: '"'}}, '"'},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(n.Children()) != 0 {
			for _, c := range n.Children() {
				if v := c.Value(); v != "\" t e s t \"" {
					t.Errorf("%q", v)
				}
			}
		} else {
			if v := n.Value(); v != "\" t e s t \"" {
				t.Errorf("%q", v)
			}
		}
	}
}

func TestParser_Reset(t *testing.T) {
	p, err := parser.New([]rune("abc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Match('a'); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Reset().Match("ab"); err != nil {
		t.Fatal(err)
	}
}

func TestParser_SetIgnoreList(t *testing.T) {
	p, err := parser.New([]rune("a\n= /* comment */ b /* comment */\n"))
	if err != nil {
		t.Fatal(err)
	}
	p.SetIgnoreList([]any{
		' ',
		op.EndOfLine{},
		op.And{"/*", op.ZeroOrMore{Value: op.AnyBut{Value: "*/"}}, "*/"},
	})
	if _, err := p.ParseEOF(op.And{'a', '=', 'b'}); err != nil {
		t.Fatal(err)
	}
	p.Reset()
	if _, err := p.ParseEOF(op.And{'a', '=', 'b'}); err != nil {
		t.Fatal(err)
	}
}
