package aptos

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/pkg/hex"
)

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	bz, err := hex.Validate(string(address))
	if err != nil {
		return err
	}
	if len(bz) != 32 {
		return fmt.Errorf("invalid aptos address %s: must be %d bytes (got %d)", address, 32, len(bz))
	}

	return nil
}
