package parser_test

import (
	"fmt"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

func ExampleNode_String() {
	a := parser.NewNode("a", "0")
	x := parser.NewParentNode("x", []*parser.Node{a})
	fmt.Println(x)
	// Output:
	// {"x": [{"a": "0"}]}
}
