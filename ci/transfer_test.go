//go:build !not_ci

package ci

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	xcclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/cordialsys/crosschain/normalize"
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
		Network:   network,
		Overrides: map[string]*setup.ChainOverride{},
		Algorithm: algorithm,
	}

	xcFactory, err := setup.LoadFactory(rpcArgs)
	require.NoError(t, err, "Failed loading factory")

	chainConfig, err := setup.LoadChain(xcFactory, rpcArgs.Chain)
	require.NoError(t, err, "Failed loading chain config")

	decimals := chainConfig.GetDecimals()
	if decimalsInput != nil {
		decimals = int32(*decimalsInput)
	}

	client, err := xcFactory.NewClient(chainConfig)
	require.NoError(t, err, "Failed creating client")

	fromWalletAddress := deriveAddress(t, xcFactory, chainConfig, fromPrivateKey)

	fmt.Println("Wallet Address:", fromWalletAddress)
	transferAmount, err := xc.NewAmountHumanReadableFromStr("0.1")
	require.NoError(t, err)
	transferAmountBlockchain := transferAmount.ToBlockchain(decimals)

	// fund multiple times, which results in multiple UTXO on utxo chains.
	fundWallet(t, chainConfig, fromWalletAddress, "0.8", contract, decimals)
	fundWallet(t, chainConfig, fromWalletAddress, "1", contract, decimals)
	fundWallet(t, chainConfig, fromWalletAddress, "1.2", contract, decimals)

	if contract != "" {
		// a bit for gas
		fundWallet(t, chainConfig, fromWalletAddress, "0.1", "", chainConfig.Decimals)
	}

	require.NoError(t, err, "Failed to fund wallet address")

	signer, err := xcFactory.NewSigner(chainConfig.Base(), fromPrivateKey)
	require.NoError(t, err)

	publicKey, err := signer.PublicKey()
	require.NoError(t, err)

	addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base())
	require.NoError(t, err)

	from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
	require.NoError(t, err)

	toAddress := deriveAddress(t, xcFactory, chainConfig, toPrivateKey)
	fmt.Println("sending from ", from, " to ", toAddress)

	tfOptions := []builder.BuilderOption{
		builder.OptionTimestamp(time.Now().Unix()),
		builder.OptionPublicKey(publicKey),
	}

	balanceArgs := xcclient.NewBalanceArgs(fromWalletAddress)
	assetId := string(chainConfig.Chain)
	if contract != "" {
		tfOptions = append(tfOptions, builder.OptionContractAddress(xc.ContractAddress(contract)))
		tfOptions = append(tfOptions, builder.OptionContractDecimals(int(decimals)))
		assetId = normalize.NormalizeAddressString(contract, chainConfig.Chain)

		balanceArgs.SetContract(xc.ContractAddress(contract))
	}

	tfArgs, err := builder.NewTransferArgs(from, toAddress, transferAmountBlockchain, tfOptions...)
	require.NoError(t, err)

	var initialBalance xc.AmountBlockchain

	fmt.Println("Wallet Balance before transaction:", initialBalance.String())
	// Because we haven't been successful with getting the faucets on devnet nodes
	// to be syncronous, we instead tolerate some delay in the test
	for attempts := range 30 {
		initialBalance, err = client.FetchBalance(context.Background(), balanceArgs)
		require.NoError(t, err, fmt.Sprintf("Failed to fetch balance on attempt %d", attempts))
		asHuman := initialBalance.ToHuman(decimals).String()
		if asHuman == "3" {
			break
		}
		time.Sleep(1 * time.Second)
	}
	require.Equal(t, "3", initialBalance.ToHuman(decimals).String(), "Failed to get balance over after 30 attempts")

	input, err := client.FetchTransferInput(context.Background(), tfArgs)
	require.NoError(t, err)

	// set params on input that are enforced by the builder (rather than depending soley on untrusted RPC)
	input, err = builder.WithTxInputOptions(input, tfArgs.GetAmount(), &tfArgs)
	require.NoError(t, err)

	fmt.Println("transfer input: ", asJson(input))

	err = xc.CheckMaxFeeLimit(input, chainConfig)
	require.NoError(t, err)

	builder, err := xcFactory.NewTxBuilder(chainConfig.Base())
	require.NoError(t, err)

	tx, err := builder.Transfer(tfArgs, input)
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

	// submitting again should work or return a detectable error
	err = client.SubmitTx(context.Background(), tx)
	if err == nil {
		// ok
		fmt.Println("No error on resubmit")
	} else {
		// should be TransactionExists
		fmt.Println("Resubmit error: ", err)
		xcErr := drivers.CheckError(chainConfig.Driver, err)
		require.Equal(t, errors.TransactionExists, xcErr)
	}

	fmt.Println("submitted tx", tx.Hash())
	start := time.Now()

	var txInfo xcclient.TxInfo

	timeout := time.Minute * 1
	for {
		if time.Since(start) > timeout {
			require.Fail(t, fmt.Sprintf("Timed out waiting %v for transactions", time.Since(start)))
		}
		time.Sleep(1 * time.Second)
		info, err := client.FetchTxInfo(context.Background(), tx.Hash())
		if err != nil {
			fmt.Printf("could not find tx yet, trying again (%v)...\n", err)
			continue
		}
		if info.Confirmations < 1 {
			fmt.Printf("waiting for 1 confirmation...\n")
			continue
		}
		finalWalletBalance, err := client.FetchBalance(context.Background(), balanceArgs)
		require.NoError(t, err, "Failed to fetch balance")
		if finalWalletBalance.String() == initialBalance.String() {
			fmt.Printf("waiting for change in balance...\n")
			continue
		}

		txInfo = info
		fmt.Println(asJson(txInfo))
		break
	}

	var finalWalletBalance xc.AmountBlockchain
	var remainder xc.AmountBlockchain

	// We poll until we the "full" expected balance change, as sometimes
	// the balance can partially update (e.g. deducts network fee first...).
	for range 50 {
		finalWalletBalance, err = client.FetchBalance(context.Background(), balanceArgs)
		require.NoError(t, err, "Failed to fetch balance")
		fmt.Printf("Balance of %s after transaction: %v\n", fromWalletAddress, finalWalletBalance)

		remainder = initialBalance
		for _, movement := range txInfo.Movements {
			if movement.AssetId != xc.ContractAddress(assetId) {
				// skip movements not matching the asset we transferred
				continue
			}
			for _, from := range movement.From {
				if from.AddressId == fromWalletAddress {
					// subtract
					remainder = remainder.Sub(&from.Balance)
				}
			}
			for _, to := range movement.To {
				if to.AddressId == fromWalletAddress {
					// add
					remainder = remainder.Add(&to.Balance)
				}
			}
		}
		if finalWalletBalance.String() == remainder.String() {
			break
		} else {
			time.Sleep(500 * time.Millisecond)
		}
	}

	require.Equal(t, finalWalletBalance.String(), remainder.String())
	require.Less(t, finalWalletBalance.Uint64(), initialBalance.Uint64())
}
