package agent

import (
	"encoding/hex"
	"net/url"
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/client/types/icp"
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

	var balance icp.Balance
	accountID, err := hex.DecodeString("9523dc824aa062dcd9c91b98f4594ff9c6af661ac96747daef2090b7fe87037d")
	require.NoError(t, err)
	err = a.CallAnonymous(icp.LedgerPrincipal, icp.MethodAccountBalance, []any{
		icp.GetBalanceArgs{Account: accountID},
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

	var balance icp.Balance
	accountID, _ := hex.DecodeString("9523dc824aa062dcd9c91b98f4594ff9c6af661ac96747daef2090b7fe87037d")
	err := a.Query(icp.LedgerPrincipal, icp.MethodAccountBalance, []any{
		icp.GetBalanceArgs{Account: accountID},
	}, []any{&balance})

	require.Equal(t, uint64(0), balance.E8S)
	require.NoError(t, err)
}
