package evm_legacy

import (
	xc "github.com/cordialsys/crosschain"
	evmaddress "github.com/cordialsys/crosschain/chain/evm/address"
)

type AddressBuilder = evmaddress.AddressBuilder

var NewAddressBuilder = evmaddress.NewAddressBuilder

var _ xc.AddressBuilder = AddressBuilder{}
