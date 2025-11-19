package substrate

import (
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate/address"
	"github.com/cordialsys/crosschain/chain/substrate/client"
	"github.com/stretchr/testify/require"
	"github.com/vedhavyas/go-subkey/v2"
)

func Validate(t *testing.T, chainCfg *xc.ChainConfig) {
	require := require.New(t)
	help := fmt.Sprintf("Invalid configuration for %s. Substrate chains must have the correct chain prefix u16 set, see https://polkadot.subscan.io/tools/format_transform",
		chainCfg.Chain,
	)

	require.NotEmpty(chainCfg.ChainPrefix, help)
	_, ok := chainCfg.ChainPrefix.AsInt()
	require.True(ok, help)

	// check indexer url
	if chainCfg.IndexerType != client.IndexerRpc {
		help = fmt.Sprintf("Invalid configuration for %s. Need to use 'rpc' or set indexer_url for supported subscan or taostats endpoint, see https://support.subscan.io/",
			chainCfg.Chain,
		)
		require.NotEmpty(chainCfg.IndexerUrl, help)
	}
}

func ValidateAddress(cfg *xc.ChainBaseConfig, addr xc.Address) error {
	addrStr := string(addr)

	_, _, err := subkey.SS58Decode(addrStr)
	if err != nil {
		return fmt.Errorf("invalid substrate address %s: %w", addr, err)
	}

	_, err = address.Decode(addr)
	if err != nil {
		return fmt.Errorf("invalid substrate address %s: %w", addr, err)
	}

	return nil
}
