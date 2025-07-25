package op

import (
	"fmt"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

// Any matches any character.
type Any struct{}

func (a Any) Match(start parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	if p.Reader.Done() {
		return start, p.NewNoMatchError(a, start, start)
	}
	return p.Reader.Next().Cursor(), nil
}

func (a Any) String() string {
	return "."
}

type AnyBut struct {
	Value any
}

func (a AnyBut) Match(_ parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	return p.Match(And{Not(a), Any{}})
}

func (a AnyBut) String() string {
	return fmt.Sprintf("!%s .", StringAny(a.Value))
}
