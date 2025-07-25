package op

import (
	"fmt"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

// ZeroOrMore matches the given expression zero or more times.
type ZeroOrMore struct {
	Value any
}

func (zero ZeroOrMore) Match(start parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	var end = start
	for {
		c, err := p.Match(zero.Value)
		if err != nil {
			break
		}
		end = c
	}
	return end, nil
}

func (zero ZeroOrMore) Parse(p *parser.Parser) (*parser.Node, error) {
	var nodes []*parser.Node
	for {
		node, err := p.Parse(zero.Value)
		if err != nil {
			break
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

func (zero ZeroOrMore) String() string {
	return fmt.Sprintf("%v*", StringAny(zero.Value))
}
