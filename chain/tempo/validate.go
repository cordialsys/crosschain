package tempo

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm"
)

// ValidateConfig delegates to EVM config validation
func ValidateConfig(chain *xc.ChainConfig) error {
	return evm.ValidateConfig(chain)
}

// ValidateAddress delegates to EVM address validation
func ValidateAddress(cfg *xc.ChainBaseConfig, addr xc.Address) error {
	return evm.ValidateAddress(cfg, addr)
}
