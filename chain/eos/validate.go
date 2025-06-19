package eos

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
	"github.com/stretchr/testify/require"
)

func Validate(t *testing.T, chainCfg *xc.ChainConfig) {
	require := require.New(t)
	for _, na := range chainCfg.NativeAssets {
		// all contract IDs should be valid eos contract IDs
		// in format "<contract>/<symbol>"
		_, _, err := tx_input.ParseContractId(&xc.ChainBaseConfig{}, na.ContractId, nil)
		require.NoError(err, "invalid contract ID: %s", na.ContractId)
	}
}
