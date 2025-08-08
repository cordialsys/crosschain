package op

import "github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"

// EndOfLine matches the end of a line.
type EndOfLine struct{}

func (e EndOfLine) Match(_ parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	return p.Match(Or{"\n\r", '\n', '\r'})
}

func (e EndOfLine) String() string {
	return `\n`
}
