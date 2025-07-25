package parser_test

import (
	"fmt"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser/op"
)

func ExampleInvalidTypeError() {
	fmt.Println(parser.NewInvalidTypeError('0'))
	// Output:
	// invalid type: int32
}

func ExampleNoMatchError() {
	p, _ := parser.New([]rune("test"))
	_, err := p.Match(op.And{'t', 'e', 's', 't', 'i', 'f', 'y'})
	fmt.Println(err)
	// Output:
	// error stack:
	// 2) [1:1/1:5] '�' | no match: ('t' 'e' 's' 't' 'i' 'f' 'y')
	// test
	// ----^
	// 1) [1:5/1:5] '�' | no match: 'i'
	// test
	// ----^
}
