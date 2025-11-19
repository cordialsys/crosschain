package ton

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/ton/address"
)

func ValidateAddress(cfg *xc.ChainBaseConfig, addr xc.Address) error {
	_, err := address.ParseAddress(addr, cfg.Network)
	if err != nil {
		return fmt.Errorf("invalid ton address %s: %w", addr, err)
	}

	return nil
}
