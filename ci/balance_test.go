//go:build ci

package ci

import (
	"flag"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/cmd/xc/commands"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/test-go/testify/require"
)

func TestBalance(t *testing.T) {
	flag.Parse()

	validateRequiredFlags(t, chain, "Chain argument is required. Use --chain flag to specify it.")
	validateRequiredFlags(t, rpc, "RPC argument is required. Use --rpc flag to specify it.")

	privateKey := "715a5f0e6adff28fb7aee4082d3763e1182a7f93c65bb407028f70b07fc2b0f9"

	rpcArgs := &setup.RpcArgs{
		Chain:     chain,
		Rpc:       rpc,
		Overrides: map[string]*setup.ChainOverride{},
	}

	xcFactory, err := setup.LoadFactory(rpcArgs)
	require.NoError(t, err, "Failed loading factory")

	chainConfig, err := setup.LoadChain(xcFactory, rpcArgs.Chain)
	require.NoError(t, err, "Failed loading chain config")

	client, err := xcFactory.NewClient(commands.AssetConfig(chainConfig, "", 0))
	require.NoError(t, err, "Failed creating client")

	walletAddress, err := commands.DeriveAddress(xcFactory, chainConfig, privateKey)
	require.NoError(t, err, "Failed generating address")

	fmt.Println("Wallet Address:", walletAddress)

	foundAmount, err := fundWallet(chainConfig, walletAddress)
	require.NoError(t, err, "Failed to fund wallet address")

	walletBalance, err := commands.RetrieveBalance(client, xc.Address(walletAddress))
	require.NoError(t, err, "Failed to retrieve wallet balance")

	fmt.Println("Wallet Balance:", walletBalance)

	require.Equal(t, walletBalance, foundAmount)
}
