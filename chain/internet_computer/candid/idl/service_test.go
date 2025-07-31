package idl_test

import (
	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/idl"
)

func ExampleService() {
	p := address.MustDecode("w7x7r-cok77-xa")
	test(
		[]idl.Type{idl.NewServiceType(
			map[string]*idl.FunctionType{
				"foo": idl.NewFunctionType(
					[]idl.FunctionParameter{{Type: new(idl.TextType)}},
					[]idl.FunctionParameter{{Type: new(idl.NatType)}},
					nil,
				),
			},
		)},
		[]any{
			p,
		},
	)
	// Output:
	// 4449444c026a0171017d00690103666f6f0001010103caffee
}
