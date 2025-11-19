package zcash

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
	"github.com/cordialsys/crosschain/chain/zcash/address"
)

func ValidateAddress(cfg *xc.ChainBaseConfig, addr xc.Address) error {
	// Get chain params
	chainParams, err := params.GetParams(cfg)
	if err != nil {
		return fmt.Errorf("invalid zcash address %s: failed to get chain params: %w", addr, err)
	}

	// Decode the address using Zcash's decoder
	decoder := address.NewAddressDecoder()
	_, err = decoder.Decode(addr, &chainParams)
	if err != nil {
		return fmt.Errorf("invalid zcash address %s: %w", addr, err)
	}

	return nil
}
