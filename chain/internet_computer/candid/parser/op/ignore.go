package op

import (
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

type Ignore struct {
	Value any
}

func (s Ignore) Match(_ parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	toggleIgnore := p.IgnoreDisabled()
	p.ToggleIgnore(true)
	defer p.ToggleIgnore(toggleIgnore)

	return p.Match(s.Value)
}

func (s Ignore) String() string {
	return StringAny(And{s.Value})
}
