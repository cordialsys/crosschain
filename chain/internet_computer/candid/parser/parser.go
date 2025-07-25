package parser

// Parser is the parser.
type Parser struct {
	// Reader is the input reader.
	Reader *Reader
	Rules  map[string]Operator

	// ignore is the list of runes to ignore.
	ignore        []any
	disableIgnore bool
}

// New creates a new parser.
func New(input []rune) (*Parser, error) {
	r, err := NewReader(input)
	if err != nil {
		return nil, err
	}
	return &Parser{
		Reader: r,
		Rules:  make(map[string]Operator),
	}, nil
}

func (p *Parser) IgnoreDisabled() bool {
	return p.disableIgnore
}

// Match the given value.
// Returns the end cursor if the match was successful.
// Returns an error if the match failed.
func (p *Parser) Match(v any) (Cursor, error) {
	if !p.disableIgnore {
		p.ignoreAll()
	}

	start := p.Reader.Cursor()

	// Match operator.
	if v, ok := v.(Operator); ok {
		return v.Match(start, p)
	}

	return p.matchPrimitive(start, v)
}

// MatchEOF matches the given value and ensures that the end of the input is reached.
func (p *Parser) MatchEOF(v any) (Cursor, error) {
	end, err := p.Match(v)
	if err != nil {
		return end, err
	}

	if !p.disableIgnore {
		p.ignoreAll()
	}
	if !p.Reader.Done() {
		return end, p.NewNoMatchError("EOF", p.Reader.Cursor(), p.Reader.Cursor())
	}
	return end, nil
}

// Parse the given value.
func (p *Parser) Parse(v any) (*Node, error) {
	if v, ok := v.(Capture); ok {
		if !p.disableIgnore {
			p.ignoreAll()
		}
		return v.Parse(p)
	}
	_, err := p.Match(v)
	return nil, err
}

// ParseEOF parses the given value and ensures that the end of the input is reached.
func (p *Parser) ParseEOF(v any) (*Node, error) {
	n, err := p.Parse(v)
	if err != nil {
		return nil, err
	}

	if !p.disableIgnore {
		p.ignoreAll()
	}
	if !p.Reader.Done() {
		return nil, p.NewNoMatchError("EOF", p.Reader.Cursor(), p.Reader.Cursor())
	}
	return n, nil
}

// Reset the parser. All state is lost.
func (p *Parser) Reset() *Parser {
	p.Reader.cursor = Cursor{
		character: p.Reader.input[0],
	}
	return p
}

func (p *Parser) SetIgnoreList(ignore []any) {
	p.ignore = ignore
}

func (p *Parser) ToggleIgnore(disable bool) {
	p.disableIgnore = disable
}

func (p *Parser) ignoreAll() {
	disableIgnore := p.disableIgnore
	p.ToggleIgnore(true)
	defer p.ToggleIgnore(disableIgnore)

	for {
		var found bool
		for _, i := range p.ignore {
			if _, iErr := p.Match(i); iErr == nil {
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
}

// matchPrimitive matches a primitive value. Supports runes and strings.
func (p *Parser) matchPrimitive(start Cursor, v any) (Cursor, error) {
	switch v := v.(type) {
	case rune:
		if start.character != v {
			return start, p.NewNoMatchError(v, start, start)
		}
		return p.Reader.Next().Cursor(), nil
	case string:
		end := start
		for _, r := range v {
			if end.character != r {
				p.Reader.Jump(start)
				return end, p.NewNoMatchError(v, start, end)
			}
			end = p.Reader.Next().Cursor()
		}
		return end, nil
	default:
		return start, NewInvalidTypeError(v)
	}
}
