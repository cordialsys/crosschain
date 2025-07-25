package parser

import "fmt"

// Cursor represents the current position in the input.
type Cursor struct {
	// character is the current character rune.
	character rune
	// position is the absolute position in the input.
	position int

	// lastNewline is the last newline absolute position.
	lastNewline int

	// line is the current line.
	line int
	// column is the current column.
	column int
}

// Character returns the current character rune.
func (c Cursor) Character() rune {
	return c.character
}

// LastNewLine returns the last end of line absolute position.
func (c Cursor) LastNewLine() int {
	return c.lastNewline
}

// Line returns the current line.
func (c Cursor) Line() (int, int) {
	return c.line, c.column
}

// Position returns the absolute position in the input.
func (c Cursor) Position() int {
	return c.position
}

// String returns the string representation of the cursor.
func (c Cursor) String() string {
	return fmt.Sprintf("[%d:%d] %q", c.line+1, c.column+1, string(c.character))
}
