package idl_test

import (
	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/idl"
)

func ExampleFunctionType() {
	test_(
		[]idl.Type{
			idl.NewFunctionType(
				[]idl.FunctionParameter{{Type: new(idl.TextType)}},
				[]idl.FunctionParameter{{Type: new(idl.NatType)}},
				nil,
			),
		},
		[]any{
			&idl.PrincipalMethod{
				Principal: address.MustDecode("w7x7r-cok77-xa"),
				Method:    "foo",
			},
		},
	)
	// Output:
	// 4449444c016a0171017d000100010103caffee03666f6f
}
