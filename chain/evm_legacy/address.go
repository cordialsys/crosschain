package evm_legacy

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm"
)

type AddressBuilder = evm.AddressBuilder

var NewAddressBuilder = evm.NewAddressBuilder

var _ xc.AddressBuilder = AddressBuilder{}
