//go:build ci

package ci

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/stretchr/testify/require"
)

func TestTransfer(t *testing.T) {
	flag.Parse()

	validateCLIInputs(t)

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

	client, err := xcFactory.NewClient(chainConfig)
	require.NoError(t, err, "Failed creating client")

	fromWalletAddress := deriveAddress(t, xcFactory, chainConfig, fromPrivateKey)

	fmt.Println("Wallet Address:", fromWalletAddress)
	transferAmount, err := xc.NewAmountHumanReadableFromStr("0.1")
	require.NoError(t, err)
	transferAmountBlockchain := transferAmount.ToBlockchain(chainConfig.Decimals)

	fundWallet(t, chainConfig, fromWalletAddress, "1")
	require.NoError(t, err, "Failed to fund wallet address")

	initialBalance, err := client.FetchBalance(context.Background(), xc.Address(fromWalletAddress))
	require.NoError(t, err, "Failed to fetch balance")

	fmt.Println("Wallet Balance before transaction:", initialBalance.String())

	require.Equal(t, "1", initialBalance.ToHuman(chainConfig.Decimals).String())

	signer, err := xcFactory.NewSigner(chainConfig, fromPrivateKey)
	require.NoError(t, err)

	publicKey, err := signer.PublicKey()
	require.NoError(t, err)

	addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig)
	require.NoError(t, err)

	from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
	require.NoError(t, err)

	toAddress := deriveAddress(t, xcFactory, chainConfig, toPrivateKey)
	fmt.Println("sending from ", from, " to ", toAddress)

	input, err := client.FetchLegacyTxInput(context.Background(), from, toAddress)
	require.NoError(t, err)

	if inputWithPublicKey, ok := input.(xc.TxInputWithPublicKey); ok {
		inputWithPublicKey.SetPublicKey(publicKey)
		fmt.Println("added public key = ", hex.EncodeToString(publicKey))
	}

	if inputWithAmount, ok := input.(xc.TxInputWithAmount); ok {
		inputWithAmount.SetAmount(transferAmountBlockchain)
	}
	fmt.Println("transfer input: ", asJson(input))

	builder, err := xcFactory.NewTxBuilder(chainConfig)
	require.NoError(t, err)

	tx, err := builder.NewTransfer(from, toAddress, transferAmountBlockchain, input)
	require.NoError(t, err)

	sighashes, err := tx.Sighashes()
	require.NoError(t, err)

	signatures := []xc.TxSignature{}
	for _, sighash := range sighashes {
		// sign the tx sighash(es)
		signature, err := signer.Sign(sighash)
		if err != nil {
			panic(err)
		}
		signatures = append(signatures, signature)
	}

	err = tx.AddSignatures(signatures...)
	require.NoError(t, err, "could not add signatures")

	err = client.SubmitTx(context.Background(), tx)
	require.NoError(t, err)

	fmt.Println("submitted tx", tx.Hash())
	start := time.Now()

	var txInfo xcclient.TxInfo
	timeout := time.Minute * 1
	for time.Since(start) < timeout {
		time.Sleep(1 * time.Second)
		info, err := client.FetchTxInfo(context.Background(), tx.Hash())
		if err != nil {
			fmt.Printf("could not find tx yet, trying again (%v)...\n", err)
			continue
		}
		txInfo = info
		fmt.Println(asJson(txInfo))
		break
	}

	finalWalletBalance, err := client.FetchBalance(context.Background(), xc.Address(fromWalletAddress))
	require.NoError(t, err, "Failed to fetch balance")

	fmt.Println("Balance after transaction:", finalWalletBalance)

	// TODO the main transfer may not necessary be reported first, we are being lazy here.
	transferredAmount := xc.NewAmountBlockchainFromStr(txInfo.Movements[0].From[0].Balance.String())

	// TODO there may be multiple fees, we are being lazy and assuming there is only one.
	transactionFee := xc.NewAmountBlockchainFromStr(txInfo.Fees[0].Balance.String())

	totalSpend := transferredAmount.Add(&transactionFee)
	remainder := initialBalance.Sub(&totalSpend)

	require.Equal(t, finalWalletBalance.String(), remainder.String())
}
