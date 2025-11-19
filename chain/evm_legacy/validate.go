package evm_legacy

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm"
)

// ValidateAddress validates an EVM legacy address by delegating to the standard EVM validation
func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	return evm.ValidateAddress(cfg, address)
}
