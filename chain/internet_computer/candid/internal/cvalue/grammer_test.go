package cvalue_test

import (
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/internal/cvalue"
)

func TestValues(t *testing.T) {
	for _, vs := range []string{
		"()",
		"(    )",

		"0",
		"( 0 )",
		"( 0 : nat8, 1_000 )",
		"( 0 : int8 )",
		"( 0 : float32 )",
		"( 0.000_001 : float64 )",

		"(true)",
		"(false : bool)",

		"null",
		"(null)",

		"\"\"",
		"(\"\")",
		"(\"Hello world.\" : text)",

		"opt 0",

		"record{}",
		"record{ f0 = 0; f1 = opt 0 }",
		"record{\n\tf0 = 0;\n\tf1 = opt 0;\n}",

		"variant{ e }",
		"variant{ e = 0; }",

		"principal \"aaaaa-aaa\"",

		"vec{}",
		"vec{ 0; 1; 2 }",
	} {
		p, err := cvalue.NewParser([]rune(vs))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.ParseEOF(cvalue.Values); err != nil {
			t.Fatal(err)
		}
	}
}
