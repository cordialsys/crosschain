package op

import (
	"fmt"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

type Reference struct {
	Name string
}

func (r Reference) Match(start parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	v, ok := p.Rules[r.Name]
	if !ok {
		return start, fmt.Errorf("rule %s not found", r.Name)
	}
	return v.Match(start, p)
}

func (r Reference) Parse(p *parser.Parser) (*parser.Node, error) {
	v, ok := p.Rules[r.Name]
	if !ok {
		return nil, fmt.Errorf("rule %s not found", r.Name)
	}
	return p.Parse(v)
}

func (r Reference) String() string {
	return r.Name
}
