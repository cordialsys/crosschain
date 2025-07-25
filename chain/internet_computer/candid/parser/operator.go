package parser

import (
	"fmt"
)

// Capture is the interface that wraps the Parse method.
// Parse parses the input and returns a node.
type Capture interface {
	Parse(p *Parser) (end *Node, err error)

	Operator
	fmt.Stringer
}

// Operator is the interface that wraps the Match method.
// March check whether the interface matches the input.
type Operator interface {
	// Match the given value. Returns a cursor to the next character if matched, the cursor of the input will be moved
	// to the end of the match. Otherwise, returns an error, the cursor of the input will not be moved.
	// If an error is returned, the returned cursor is the last matched cursor.
	Match(start Cursor, p *Parser) (end Cursor, err error)

	fmt.Stringer
}
