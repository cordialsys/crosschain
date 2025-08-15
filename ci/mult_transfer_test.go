//go:build !not_ci

package ci

import (
	"context"
	"encoding/hex"
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
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/cordialsys/crosschain/normalize"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestMultiTransfer(t *testing.T) {
	flag.Parse()
	validateCLIInputs(t)

	// Define multiple private keys for testing
	fromPrivateKeys := []string{
		"93a4def9eb501965b9f5f3079fab53284ea6a557e48e8affa817ab0258908bbc",
		"22194a8955e9233aa2f0a0206c8ea861e5fa92a613ab5c7e236a11de3f4bc9ad",
	}
	toPrivateKeys := []string{
		"6a4aed1042d10d3102281cfc9fe94e8584dc5f089bffbad0c497c10f6deb4d7d",
		"7b4aed1042d10d3102281cfc9fe94e8584dc5f089bffbad0c497c10f6deb4d7e",
	}
	feePayerPrivateKey := "8c4aed1042d10d3102281cfc9fe94e8584dc5f089bffbad0c497c10f6deb4d7f"

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

	// Derive addresses for all participants
	fromAddresses := make([]xc.Address, len(fromPrivateKeys))
	for i, pk := range fromPrivateKeys {
		fromAddresses[i] = deriveAddress(t, xcFactory, chainConfig, pk)
	}

	toAddresses := make([]xc.Address, len(toPrivateKeys))
	for i, pk := range toPrivateKeys {
		toAddresses[i] = deriveAddress(t, xcFactory, chainConfig, pk)
	}

	feePayerWalletAddress := deriveAddress(t, xcFactory, chainConfig, feePayerPrivateKey)

	fmt.Println("From Addresses:", fromAddresses)
	fmt.Println("To Addresses:", toAddresses)
	fmt.Println("Fee Payer Address:", feePayerWalletAddress)

	// Define transfer amounts
	transferAmounts := []xc.AmountBlockchain{
		xc.NewAmountHumanReadableFromFloat(0.1).ToBlockchain(chainConfig.GetDecimals()),
		xc.NewAmountHumanReadableFromFloat(0.2).ToBlockchain(chainConfig.GetDecimals()),
	}

	// Fund wallets for gas if needed
	if feePayer {
		fundWallet(t, chainConfig, feePayerWalletAddress, "0.1", "", chainConfig.Decimals)
	} else {
		if contract != "" {
			for _, addr := range fromAddresses {
				fundWallet(t, chainConfig, addr, "0.1", "", chainConfig.Decimals)
			}
		}
	}
	balanceArgs := make([]*xcclient.BalanceArgs, len(fromAddresses))
	for i, addr := range fromAddresses {
		balanceArgs[i] = xcclient.NewBalanceArgs(addr)
	}
	assetId := string(chainConfig.Chain)
	if contract != "" {
		assetId = normalize.NormalizeAddressString(contract, chainConfig.Chain)
		for _, balanceArg := range balanceArgs {
			balanceArg.SetContract(xc.ContractAddress(contract))
		}
	}

	// Fund the source wallets
	expectedFundingTotal := 0.0
	for i, addr := range fromAddresses {
		// use a variey of UTXO's
		fundWallet(t, chainConfig, addr, "0.4", contract, decimals)
		fundWallet(t, chainConfig, addr, "0.6", contract, decimals)
		expectedFundingTotal += 0.4 + 0.6
		for range i {
			fundWallet(t, chainConfig, addr, "0.01", contract, decimals)
			expectedFundingTotal += 0.01
		}
	}

	// Create signers collection
	signers := signer.NewCollection()

	// Add signers
	for i, pk := range fromPrivateKeys {
		signer, err := xcFactory.NewSigner(chainConfig.Base(), pk)
		require.NoError(t, err)
		signers.AddAuxSigner(signer, fromAddresses[i])
	}

	// Add fee payer if needed
	if feePayer {
		feePayerSigner, err := xcFactory.NewSigner(chainConfig.Base(), feePayerPrivateKey)
		require.NoError(t, err)
		signers.AddAuxSigner(feePayerSigner, feePayerWalletAddress)
	}

	// Create senders
	senders := make([]*builder.Sender, len(fromAddresses))
	for i, addr := range fromAddresses {
		signer, err := xcFactory.NewSigner(chainConfig.Base(), fromPrivateKeys[i])
		require.NoError(t, err)
		publicKey, err := signer.PublicKey()
		require.NoError(t, err)
		senders[i], err = builder.NewSender(addr, publicKey)
		require.NoError(t, err)
	}

	// Create receivers
	receivers := make([]*builder.Receiver, len(toAddresses))
	for i, addr := range toAddresses {
		options := []builder.BuilderOption{}
		if contract != "" {
			options = append(options, builder.OptionContractAddress(xc.ContractAddress(contract)))
			options = append(options, builder.OptionContractDecimals(int(decimals)))
		}
		receivers[i], err = builder.NewReceiver(addr, transferAmounts[i], options...)
		require.NoError(t, err)
	}

	// Create transaction options
	tfOptions := []builder.BuilderOption{
		builder.OptionTimestamp(time.Now().Unix()),
	}

	if feePayer {
		feePayerSigner, err := xcFactory.NewSigner(chainConfig.Base(), feePayerPrivateKey)
		require.NoError(t, err)
		publicKey, err := feePayerSigner.PublicKey()
		require.NoError(t, err)
		tfOptions = append(tfOptions, builder.OptionFeePayer(feePayerWalletAddress, publicKey))
	}

	initialBalance := xc.NewAmountHumanReadableFromFloat(expectedFundingTotal).ToBlockchain(decimals)
	awaitBalance(t, client, initialBalance, decimals, balanceArgs...)

	// Create multi-transfer arguments
	tfArgs, err := builder.NewMultiTransferArgs(chainConfig.Base(), senders, receivers, tfOptions...)
	require.NoError(t, err)

	// Get multi-transfer client
	multiClient, ok := client.(xcclient.MultiTransferClient)
	require.True(t, ok, "Client does not support multi-transfer")

	// Fetch input for multi-transfer
	input, err := multiClient.FetchMultiTransferInput(context.Background(), *tfArgs)
	require.NoError(t, err)

	// Check fee limit
	err = xc.CheckFeeLimit(input, chainConfig)
	require.NoError(t, err)

	// Get transaction builder
	txBuilder, err := xcFactory.NewTxBuilder(chainConfig.Base())
	require.NoError(t, err)

	// Check if builder supports multi-transfer
	multiTxBuilder, ok := txBuilder.(builder.MultiTransfer)
	require.True(t, ok, "Transaction builder does not support multi-transfer")

	// Build transaction
	tx, err := multiTxBuilder.MultiTransfer(*tfArgs, input)
	require.NoError(t, err)

	// Get sighashes
	sighashes, err := tx.Sighashes()
	require.NoError(t, err)

	// Sign transaction
	signatures := []*xc.SignatureResponse{}
	for _, sighash := range sighashes {
		signature, err := signers.Sign(sighash.Signer, sighash.Payload)
		require.NoError(t, err)
		signatures = append(signatures, signature)
	}

	// Add signatures to transaction
	err = tx.SetSignatures(signatures...)
	require.NoError(t, err)

	if txMoreSigs, ok := tx.(xc.TxAdditionalSighashes); ok {
		for {
			additionalSighashes, err := txMoreSigs.AdditionalSighashes()
			require.NoError(t, err, "could not get additional sighashes")
			if len(additionalSighashes) == 0 {
				break
			}
			for _, additionalSighash := range additionalSighashes {
				log := logrus.WithField("payload", hex.EncodeToString(additionalSighash.Payload))
				signature, err := signers.Sign(additionalSighash.Signer, additionalSighash.Payload)
				if err != nil {
					panic(err)
				}
				signatures = append(signatures, signature)
				log.
					WithField("address", signature.Address).
					WithField("signature", hex.EncodeToString(signature.Signature)).Info("adding additional signature")
			}
			err = tx.SetSignatures(signatures...)
			require.NoError(t, err, "could set signatures")
		}
	}

	// Submit transaction
	err = client.SubmitTx(context.Background(), tx)
	require.NoError(t, err)

	// Try submitting again to check for TransactionExists error
	err = client.SubmitTx(context.Background(), tx)
	if err == nil {
		fmt.Println("No error on resubmit")
	} else {
		fmt.Println("Resubmit error: ", err)
		xcErr := drivers.CheckError(chainConfig.Driver, err)
		require.Equal(t, errors.TransactionExists, xcErr)
	}

	fmt.Println("submitted tx", tx.Hash())
	txInfo := awaitTx(t, client, tx.Hash(), initialBalance, balanceArgs...)

	// Verify fee payer movement if used
	if feePayer {
		found := false
		for _, movement := range txInfo.Movements {
			for _, from := range movement.From {
				if from.AddressId == feePayerWalletAddress {
					found = true
					break
				}
			}
		}
		require.True(t, found, "Fee payer movement not found")
	}

	// Verify all expected transfers occurred
	for i, toAddr := range toAddresses {
		found := false
		for _, movement := range txInfo.Movements {
			for _, to := range movement.To {
				if to.AddressId == toAddr {
					found = true
					require.Equal(t, transferAmounts[i].String(), to.Balance.String(),
						"Transfer amount mismatch for recipient %s", toAddr)
					break
				}
			}
			if found {
				break
			}
		}
		require.True(t, found, "Transfer to %s not found in transaction movements", toAddr)
	}
	verifyBalanceChanges(t, client, txInfo, assetId, initialBalance, balanceArgs...)
}
