package bitcoin

import (
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
)

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	params, err := params.GetParams(cfg)
	if err != nil {
		return fmt.Errorf("unknown bitcoin chain %s: %w", cfg.Chain, err)
	}
	_, err = btcutil.DecodeAddress(string(address), &params)
	if err != nil {
		return fmt.Errorf("invalid bitcoin address %s: %w", address, err)
	}
	return nil
}
