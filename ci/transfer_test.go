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

func TestTransfer(t *testing.T) {
	flag.Parse()

	validateRequiredFlags(t, chain, "Chain argument is required. Use --chain flag to specify it.")
	validateRequiredFlags(t, rpc, "RPC argument is required. Use --rpc flag to specify it.")

	fromPrivateKey := "93a4def9eb501965b9f5f3079fab53284ea6a557e48e8affa817ab0258908bbc"
	toPrivateKey := "22194a8955e9233aa2f0a0206c8ea861e5fa92a613ab5c7e236a11de3f4bc9ad"

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

	fromWalletAddress, err := commands.DeriveAddress(xcFactory, chainConfig, fromPrivateKey)
	require.NoError(t, err, "Failed generating address")

	fmt.Println("Wallet Address:", fromWalletAddress)

	foundAmount, err := fundWallet(chainConfig, fromWalletAddress)
	require.NoError(t, err, "Failed to fund wallet address")

	initialWalletBalance, err := commands.RetrieveBalance(client, xc.Address(fromWalletAddress))
	require.NoError(t, err, "Failed to retrieve wallet balance")

	fmt.Println("Wallet Balance before transaction:", initialWalletBalance)

	require.Equal(t, initialWalletBalance, foundAmount)

	txTransfer, err := getTxTransfer(xcFactory, chainConfig, client, fromPrivateKey, toPrivateKey)
	require.NoError(t, err, "Failed to get txTransfer")

	parsedTxTransfer, err := parseTxTransaction(txTransfer)
	require.NoError(t, err, "Failed to parse txTransfer")

	fmt.Println("Transaction:", txTransfer)

	if parsedTxTransfer.Hash != "" {
		fmt.Printf("Transaction was successful with hash: %s\n", parsedTxTransfer.Hash)
	} else {
		fmt.Println("Transaction failed")
	}

	finalWalletBalance, err := commands.RetrieveBalance(client, xc.Address(fromWalletAddress))
	require.NoError(t, err, "Failed to retrieve wallet balance")

	fmt.Println("Wallet Balance after transaction:", finalWalletBalance)

	balanceAfterTransfer, err := computeBalanceAfterTransfer(initialWalletBalance, parsedTxTransfer)
	require.NoError(t, err, "Failed to compute balance after transfer")

	require.Equal(t, finalWalletBalance, balanceAfterTransfer)
}
