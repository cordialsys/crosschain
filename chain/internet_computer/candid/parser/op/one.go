package op

import (
	"fmt"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

type OneOrMore struct {
	Value any
}

func (one OneOrMore) Match(start parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	var end parser.Cursor // Last matched cursor.
	var err error
	if end, err = p.Match(one.Value); err != nil {
		return end, parser.NewErrorStack(p.NewNoMatchError(one, start, end), err)
	}
	for {
		c, err := p.Match(one.Value)
		if err != nil {
			break
		}
		end = c
	}
	return end, nil
}

func (one OneOrMore) Parse(p *parser.Parser) (*parser.Node, error) {
	var nodes []*parser.Node
	node, err := p.Parse(one.Value)
	if err != nil {
		return nil, err
	}
	if node != nil {
		if node.Name == "" {
			nodes = append(nodes, node.Children()...)
		} else {
			nodes = append(nodes, node)
		}
	}
	for {
		node, err := p.Parse(one.Value)
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

func (one OneOrMore) String() string {
	return fmt.Sprintf("%v+", StringAny(one.Value))
}
