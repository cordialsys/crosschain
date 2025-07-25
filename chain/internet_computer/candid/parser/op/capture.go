package op

import (
	"fmt"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

// Capture is a named expression.
type Capture struct {
	// Name is required, it will otherwise be ignored while flattening the AST.
	Name string
	// Value is the expression to capture.
	Value any
}

func (c Capture) Match(_ parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	return p.Match(c.Value)
}

func (c Capture) Parse(p *parser.Parser) (*parser.Node, error) {
	start := p.Reader.Cursor()
	node, err := p.Parse(c.Value)
	if err != nil {
		return nil, err
	}
	if node != nil {
		// Set the name of the node.
		if node.Name == "" {
			node.Name = c.Name
			return node, nil
		}
		return parser.NewParentNode(c.Name, []*parser.Node{node}), nil
	}
	return parser.NewNode(c.Name, string(p.Reader.GetInputRange(start, p.Reader.Cursor()))), nil
}

func (c Capture) String() string {
	if c.Name == "" {
		return fmt.Sprintf("{%s}", StringAny(c.Value))
	}
	return c.Name
}
