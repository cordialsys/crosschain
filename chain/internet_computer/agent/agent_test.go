package agent

import (
	"encoding/hex"
	"net/url"
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	types "github.com/cordialsys/crosschain/chain/internet_computer/client/types"
	"github.com/stretchr/testify/require"
)

func TestCallAnonymous(t *testing.T) {
	u, _ := url.Parse("https://icp-api.io")
	config := AgentConfig{
		Identity:      address.Ed25519Identity{},
		IngressExpiry: 0,
		Url:           u,
		Logger:        nil,
	}
	a, _ := NewAgent(config)

	var balance types.IcpBalance
	accountID, _ := hex.DecodeString("9523dc824aa062dcd9c91b98f4594ff9c6af661ac96747daef2090b7fe87037d")
	err := a.Query(types.IcpLedgerPrincipal, types.MethodAccountBalance, []any{
		types.BalanceArgs{Account: accountID},
	}, []any{&balance})

	require.Equal(t, uint64(0), balance.E8S)
	require.NoError(t, err)
}

func TestQuery(t *testing.T) {
	u, _ := url.Parse("https://icp-api.io")
	config := AgentConfig{
		Identity:      address.Ed25519Identity{},
		IngressExpiry: 0,
		Url:           u,
		Logger:        nil,
	}
	a, _ := NewAgent(config)

	var balance types.IcpBalance
	accountID, _ := hex.DecodeString("9523dc824aa062dcd9c91b98f4594ff9c6af661ac96747daef2090b7fe87037d")
	err := a.Query(types.IcpLedgerPrincipal, types.MethodAccountBalance, []any{
		types.BalanceArgs{Account: accountID},
	}, []any{&balance})

	require.Equal(t, uint64(0), balance.E8S)
	require.NoError(t, err)
}
