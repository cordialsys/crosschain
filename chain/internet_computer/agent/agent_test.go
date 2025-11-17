package agent

import (
	"context"
	"encoding/hex"
	"net/url"
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/client/types/icp"
	"github.com/stretchr/testify/require"
)

func TestQuery(t *testing.T) {
	u, _ := url.Parse("https://icp-api.io")
	config := NewAgentConfig()
	config.SetUrl(u)
	a, _ := NewAgent(config)

	var balance icp.Balance
	accountID, _ := hex.DecodeString("9523dc824aa062dcd9c91b98f4594ff9c6af661ac96747daef2090b7fe87037d")
	err := a.Query(context.TODO(), icp.LedgerPrincipal, icp.MethodAccountBalance, []any{
		icp.GetBalanceArgs{Account: accountID},
	}, []any{&balance})

	require.Equal(t, uint64(0), balance.E8S)
	require.NoError(t, err)
}
