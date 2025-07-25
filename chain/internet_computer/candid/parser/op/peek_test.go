package op_test

import (
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser/op"
)

func TestPeek(t *testing.T) {
	p, err := parser.New([]rune("abc"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range "abc" {
		for i := 0; i < 3; i++ {
			if _, err := p.Match(op.Peek{Value: c}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := p.Match(op.Any{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := p.Match(op.EOF{}); err != nil {
		t.Fatal(err)
	}
}
