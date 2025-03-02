//go:build ci

package ci

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/stretchr/testify/require"
)

func TestBalance(t *testing.T) {
	flag.Parse()

	validateCLIInputs(t)

	privateKey := "715a5f0e6adff28fb7aee4082d3763e1182a7f93c65bb407028f70b07fc2b0f9"

	rpcArgs := &setup.RpcArgs{
		Chain:     chain,
		Rpc:       rpc,
		Network:   network,
		Overrides: map[string]*setup.ChainOverride{},
		Algorithm: algorithm,
	}

	xcFactory, err := setup.LoadFactory(rpcArgs)
	require.NoError(t, err, "Failed loading factory")

	chainConfig, err := setup.LoadChain(xcFactory, rpcArgs.Chain)
	require.NoError(t, err, "Failed loading chain config")

	client, err := xcFactory.NewClient(chainConfig)
	require.NoError(t, err, "Failed creating client")

	walletAddress := deriveAddress(t, xcFactory, chainConfig, privateKey)

	fmt.Println("Wallet Address:", walletAddress)

	fundWallet(t, chainConfig, walletAddress, "1")

	balanceArgs := xcclient.NewBalanceArgs(walletAddress)
	var walletBalance xc.AmountBlockchain

	// Tolerate ~30s to get the target balance, as the faucet for a devnet node isn't always syncronous
	for attempts := range 30 {
		walletBalance, err = client.FetchBalance(context.Background(), balanceArgs)
		require.NoError(t, err, fmt.Sprintf("Failed to fetch balance on attempt %d", attempts))
		fmt.Println("Wallet Balance: ", walletBalance)
		asHuman := walletBalance.ToHuman(chainConfig.Decimals).String()
		if asHuman == "1" {
			break
		}
		time.Sleep(1 * time.Second)
	}

	require.Equal(t, "1", walletBalance.ToHuman(chainConfig.Decimals).String())
}
