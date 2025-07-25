package parser

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorStack is a stack of errors.
type ErrorStack struct {
	Errors []error
}

// NewErrorStack returns a new ErrorStack.
func NewErrorStack(context, err error) *ErrorStack {
	var e *ErrorStack
	if errors.As(err, &e) {
		return e.AddError(context)
	}
	return &ErrorStack{
		Errors: []error{err, context},
	}
}

// AddError adds an error to the stack.
func (e *ErrorStack) AddError(err error) *ErrorStack {
	e.Errors = append(e.Errors, err)
	return e
}

// Error returns the error message.
func (e *ErrorStack) Error() string {
	var stack []string
	for i := len(e.Errors) - 1; i >= 0; i-- {
		stack = append(stack, fmt.Sprintf("%d) %s", i+1, e.Errors[i].Error()))
	}
	return fmt.Sprintf("error stack:\n%s", strings.Join(stack, "\n"))
}

// InvalidTypeError is returned when an invalid type is passed to a function.
type InvalidTypeError struct {
	V any
}

// NewInvalidTypeError returns a new InvalidTypeError.
func NewInvalidTypeError(v any) *InvalidTypeError {
	return &InvalidTypeError{V: v}
}

// Error returns the error message.
func (e *InvalidTypeError) Error() string {
	return fmt.Sprintf("invalid type: %T", e.V)
}

// NoMatchError is returned when a rule does not match.
type NoMatchError struct {
	// Operator, string, or rune.
	V any
	// Start cursor of the rule.
	Start Cursor
	// End cursor of the rule.
	End Cursor
	// Reference to the parser to get the line.
	p *Parser
}

// Error returns the error message.
func (e *NoMatchError) Error() string {
	l := string(e.p.Reader.GetLine(e.End))
	p := strings.Repeat("-", int(e.End.column))
	switch e.V.(type) {
	case rune, string:
		return fmt.Sprintf(
			"[%d:%d/%d:%d] %q | no match: %q\n%s\n%s^",
			e.Start.line+1, e.Start.column+1, e.End.line+1, e.End.column+1, e.End.character,
			e.V, l, p,
		)
	default:
		return fmt.Sprintf(
			"[%d:%d/%d:%d] %q | no match: %v\n%s\n%s^",
			e.Start.line+1, e.Start.column+1, e.End.line+1, e.End.column+1, e.End.character,
			e.V, l, p,
		)
	}
}

// NewNoMatchError returns a new NoMatchError.
func (p *Parser) NewNoMatchError(v any, start, end Cursor) *NoMatchError {
	return &NoMatchError{
		V:     v,
		Start: start,
		End:   end,
		p:     p,
	}
}
