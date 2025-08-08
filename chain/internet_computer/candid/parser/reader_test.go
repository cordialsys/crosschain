package parser_test

import (
	"testing"

	. "github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

func TestReader(t *testing.T) {
	t.Run("NewReader", func(t *testing.T) {
		// Test valid reader.
		if _, err := NewReader([]rune("test")); err != nil {
			t.Fatal(err)
		}

		// Test invalid reader.
		if _, err := NewReader(nil); err == nil {
			t.Fatal("expected error")
		}
		if _, err := NewReader([]rune("")); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("Cursor", func(t *testing.T) {
		str := "test"
		r, _ := NewReader([]rune("test"))
		for i, c := range str {
			if r.Cursor().Position() != i || r.Cursor().Character() != c {
				t.Fatalf("invalid cursor: %s", r.Cursor())
			}
			r.Next()
		}
		if r.Cursor().Position() != len(str) || r.Cursor().Character() != ReaderDone {
			t.Fatalf("invalid cursor: %s", r.Cursor())
		}
	})

	t.Run("Done", func(t *testing.T) {
		r, _ := NewReader([]rune("test"))
		for range "test" {
			r.Next()
		}
		if !r.Done() {
			t.Fatal("expected done")
		}
	})

	t.Run("Jump", func(t *testing.T) {
		str := "test"
		r, _ := NewReader([]rune(str))
		var cursors []Cursor
		for range str {
			cursors = append(cursors, r.Cursor())
			r.Next()
		}
		for i, c := range cursors {
			r.Jump(c)
			if r.Cursor() != cursors[i] {
				t.Fatalf("invalid cursor: %s", r.Cursor())
			}
		}
		r.Next()
		c := r.Cursor()
		r.Jump(c)
		if !r.Done() {
			t.Fatal("expected done")
		}
	})
}

func TestReader_Cursor(t *testing.T) {
	for _, test := range []struct {
		input                  string
		position, line, column int
		lastNl                 int
	}{
		{
			input:    "test",
			position: 4,
			line:     0,
			column:   4, // EOF
			lastNl:   0,
		},
		{
			input:    "\ntest",
			position: 5,
			line:     1,
			column:   4, // EOF
			lastNl:   1,
		},
		{
			input:    "\r\ntest",
			position: 6,
			line:     1,
			column:   4, // EOF
			lastNl:   2,
		},
		{
			input:    "\n\r\n\rtest", // Combine \n, \r\n and \r.
			position: 8,
			line:     3,
			column:   4, // EOF
			lastNl:   4,
		},
	} {
		r, err := NewReader([]rune(test.input))
		if err != nil {
			t.Fatal(err)
		}
		for !r.Done() {
			r.Next()
		}
		if r.Cursor().Position() != test.position {
			t.Fatalf("invalid position: expected %d, got %d", test.position, r.Cursor().Position())
		}
		if line, column := r.Cursor().Line(); line != test.line || column != test.column {
			t.Fatalf("invalid line/column: expected %d:%d, got %d:%d", test.line, test.column, line, column)
		}
		if r.Cursor().LastNewLine() != test.lastNl {
			t.Fatalf("invalid lastEOL: %d", r.Cursor().LastNewLine())
		}
	}
}

func TestReader_GetInputRange(t *testing.T) {
	input := []rune("abcdef")
	r, err := NewReader(input)
	if err != nil {
		t.Fatal(err)
	}
	start := r.Cursor()
	for i, c := range input {
		if r.Rune() != c {
			t.Fatalf("expected %c, got %c", c, r.Rune())
		}
		inputRange := string(r.GetInputRange(start, r.Cursor()))
		if expected := string(input[:i]); inputRange != expected {
			t.Fatalf("expected %s, got %s", expected, inputRange)
		}
		r.Next()
	}
	if !r.Done() {
		t.Fatalf("expected done")
	}
	if c := r.Rune(); c != ReaderDone {
		t.Errorf("expected %c, got %c", ReaderDone, c)
	}
}

func TestReader_GetLineUntil(t *testing.T) {
	for _, test := range []struct {
		input string
		line  string
	}{
		{
			input: "test",
			line:  "test",
		},
		{
			input: "\ntest",
			line:  "test",
		},
		{
			input: "\r\ntest",
			line:  "test",
		},
		{
			input: "\n\r\n\rtest",
			line:  "test",
		},
		{
			input: "\n\r\n\rtest\n",
			line:  "test",
		},
		{
			input: "\n\r\n\rtest\n\roops",
			line:  "test",
		},
		{
			input: "\n\n\n\n\n",
		},
	} {
		r, err := NewReader([]rune(test.input))
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 4; i++ {
			r.Next()
		}
		if line := string(r.GetLine(r.Cursor())); line != test.line {
			t.Fatalf("invalid line: %q", line)
		}
	}
}
