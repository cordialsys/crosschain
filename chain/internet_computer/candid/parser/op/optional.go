package op

import (
	"fmt"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

// Optional matches the given value or nothing.
type Optional struct {
	Value any
}

func (o Optional) Match(start parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	if end, err := p.Match(o.Value); err == nil {
		return end, nil
	}
	return start, nil
}

func (o Optional) Parse(p *parser.Parser) (*parser.Node, error) {
	if node, err := p.Parse(o.Value); err == nil {
		return node, nil
	}
	return nil, nil
}

func (o Optional) String() string {
	return fmt.Sprintf("%v?", StringAny(o.Value))
}
