package op

import (
	"fmt"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

// RuneRange matches a range of runes, inclusive.
type RuneRange struct {
	// Min is the minimum rune in the range.
	Min rune
	// Max is the maximum rune in the range.
	Max rune
}

func (r RuneRange) Match(start parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	c := start.Character()
	if c < r.Min || r.Max < c {
		return start, p.NewNoMatchError(r, start, start)
	}
	return p.Reader.Next().Cursor(), nil
}

func (r RuneRange) String() string {
	return fmt.Sprintf("[%c-%c]", r.Min, r.Max)
}
