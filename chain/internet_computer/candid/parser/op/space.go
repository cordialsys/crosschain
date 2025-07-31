package op

import "github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"

// Space matches a space character, a tab character or a line break.
type Space struct{}

func (s Space) Match(_ parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	return p.Match(Or{' ', '\t', EndOfLine{}})
}

func (s Space) String() string {
	return `" "`
}
