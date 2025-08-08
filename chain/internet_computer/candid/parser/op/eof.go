package op

import (
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

// EOF matches the end of the input.
type EOF struct{}

func (eof EOF) Match(start parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	if !p.Reader.Done() {
		return start, p.NewNoMatchError(eof, start, p.Reader.Cursor())
	}
	return start, nil
}

func (eof EOF) String() string {
	return "!."
}
