package op

import (
	"fmt"
	"strings"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

type Or []any

func (or Or) Match(start parser.Cursor, p *parser.Parser) (parser.Cursor, error) {
	var end parser.Cursor // Last matched cursor.
	var err error
	for _, r := range or {
		if end, err = p.Match(r); err == nil {
			return end, nil
		}
	}
	// Will only include the last error, all other errors are ignored.
	return start, parser.NewErrorStack(p.NewNoMatchError(or, start, end), err)
}

func (or Or) Parse(p *parser.Parser) (*parser.Node, error) {
	start := p.Reader.Cursor()
	var node *parser.Node
	var err error
	for _, r := range or {
		if node, err = p.Parse(r); err == nil {
			return node, nil
		}
	}
	return nil, parser.NewErrorStack(p.NewNoMatchError(or, start, start), err)
}

func (or Or) String() string {
	if len(or) == 0 {
		return ""
	}
	if len(or) == 1 {
		StringAny(or[0])
	}
	var str []string
	for _, v := range or {
		str = append(str, StringAny(v))
	}
	return fmt.Sprintf("(%s)", strings.Join(str, " | "))
}
