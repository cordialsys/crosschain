package hedera

import (
	"fmt"
	"strings"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm"
	"github.com/cordialsys/crosschain/chain/hedera/common_types"
	"github.com/stretchr/testify/require"
)

func Validate(t *testing.T, chainCfg *xc.ChainConfig) {
	require := require.New(t)
	require.NotEmpty(chainCfg.ChainID, "chain_id should be set to grpc node id")
}

func ValidateAddress(cfg *xc.ChainBaseConfig, addr xc.Address) error {
	addrStr := string(addr)
	if strings.HasPrefix(addrStr, "0x") {
		return evm.ValidateAddress(cfg, addr)
	}

	_, err := commontypes.NewHederaAccountId(addrStr)
	if err != nil {
		return fmt.Errorf("invalid hedera account id: %w", err)
	}

	return nil
}
