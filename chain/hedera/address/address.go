package address

import (
	xc "github.com/cordialsys/crosschain"
	evmaddress "github.com/cordialsys/crosschain/chain/evm/address"
)

// NewAddressBuilder creates a new Hedera AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return evmaddress.NewAddressBuilder(cfgI)
}
