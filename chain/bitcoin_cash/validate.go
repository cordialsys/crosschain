package bitcoin_cash

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
)

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	params, err := params.GetParams(cfg)
	if err != nil {
		return fmt.Errorf("unknown bitcoin cash chain %s: %w", cfg.Chain, err)
	}

	decoder := NewAddressDecoder()
	_, err = decoder.Decode(address, &params)
	if err != nil {
		return fmt.Errorf("invalid bitcoin cash address %s: %w", address, err)
	}
	return nil
}
