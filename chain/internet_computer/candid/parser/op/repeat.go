package op

import (
	"fmt"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

type Repeat struct {
	Min   uint
	Max   int
	Value any
}

func (r Repeat) Match(start parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	end := start
	for i := 0; i < int(r.Min); i++ {
		var err error
		if end, err = p.Match(r.Value); err != nil {
			p.Reader.Jump(start)
			return start, err
		}
	}
	if r.Max != int(r.Min) {
		for i := int(r.Min); r.Max <= 0 || i < r.Max; i++ {
			c, err := p.Match(r.Value)
			if err != nil {
				break
			}
			end = c
		}
	}
	return end, nil
}

func (r Repeat) Parse(p *parser.Parser) (*parser.Node, error) {
	start := p.Reader.Cursor()
	var nodes []*parser.Node
	for i := 0; i < int(r.Min); i++ {
		node, err := p.Parse(r.Value)
		if err != nil {
			p.Reader.Jump(start)
			return nil, err
		}
		if node != nil {
			if node.Name == "" {
				nodes = append(nodes, node.Children()...)
			} else {
				nodes = append(nodes, node)
			}
		}
	}
	if r.Max != int(r.Min) {
		for i := int(r.Min); r.Max <= 0 || i < r.Max; i++ {
			node, err := p.Parse(r.Value)
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
	}
	if len(nodes) == 0 {
		return nil, nil
	}
	if len(nodes) == 1 {
		return nodes[0], nil
	}
	return parser.NewParentNode("", nodes), nil
}

func (r Repeat) String() string {
	if r.Max <= 0 {
		return fmt.Sprintf("%v{%d,}", StringAny(r.Value), r.Min)
	}
	if r.Max == int(r.Min) {
		return fmt.Sprintf("%v{%d}", StringAny(r.Value), r.Min)
	}
	return fmt.Sprintf("%v{%d,%d}", StringAny(r.Value), r.Min, r.Max)
}
