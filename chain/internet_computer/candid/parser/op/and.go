package op

import (
	"fmt"
	"strings"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

// And is a sequence of rules that must all match.
type And []any

func (and And) Match(start parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	end := start // Last matched cursor.
	for _, r := range and {
		var err error
		if end, err = p.Match(r); err != nil {
			p.Reader.Jump(start) // Reset the reader.
			return end, parser.NewErrorStack(p.NewNoMatchError(and, start, end), err)
		}
	}
	return end, nil
}

func (and And) Parse(p *parser.Parser) (*parser.Node, error) {
	start := p.Reader.Cursor()
	var nodes []*parser.Node
	for _, r := range and {
		end := p.Reader.Cursor()
		node, err := p.Parse(r)
		if err != nil {
			p.Reader.Jump(start)
			return nil, parser.NewErrorStack(p.NewNoMatchError(and, start, end), err)
		}
		if node != nil {
			if node.Name == "" {
				nodes = append(nodes, node.Children()...)
			} else {
				nodes = append(nodes, node)
			}
		}
	}
	if len(nodes) == 0 {
		return nil, nil
	}
	if len(nodes) == 1 {
		return nodes[0], nil
	}
	return parser.NewParentNode("", nodes), nil
}

func (and And) String() string {
	if len(and) == 0 {
		return ""
	}
	if len(and) == 1 {
		StringAny(and[0])
	}
	var str []string
	for _, v := range and {
		str = append(str, StringAny(v))
	}
	return fmt.Sprintf("(%s)", strings.Join(str, " "))
}
