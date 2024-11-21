package commands

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/factory"
	"github.com/sirupsen/logrus"
)

func RetrieveBalance(client xclient.Client, address xc.Address) (string, error) {
	balance, err := client.FetchBalance(context.Background(), address)
	if err != nil {
		return "", fmt.Errorf("could not fetch balance for address %s: %v", address, err)
	}

	return balance.String(), nil
}

func RetrieveTxInput(client xclient.Client, fromAddress xc.Address, toAddress xc.Address) (string, error) {
	input, err := client.FetchLegacyTxInput(context.Background(), fromAddress, toAddress)
	if err != nil {
		return "", fmt.Errorf("could not fetch transaction inputs: %v", err)
	}

	bz, _ := json.MarshalIndent(input, "", "  ")

	return string(bz), nil
}

func RetrieveTxInfo(client xclient.Client, hash string) (string, error) {
	input, err := client.FetchTxInfo(context.Background(), xc.TxHash(hash))
	if err != nil {
		return "", fmt.Errorf("could not fetch tx info: %v", err)
	}

	bz, _ := json.MarshalIndent(input, "", "  ")

	return string(bz), nil
}

func RetrieveTxTransfer(xcFactory *factory.Factory, chainConfig *xc.ChainConfig,
	contract string, memo string, timeout time.Duration, toWalletAddress string,
	amountToTransfer string, decimals int32, privateKeyInput string, client xclient.Client) (string, error) {
	transferredAmountHuman, err := xc.NewAmountHumanReadableFromStr(amountToTransfer)
	if err != nil {
		return "", err
	}

	amountBlockchain := transferredAmountHuman.ToBlockchain(decimals)

	signer, err := xcFactory.NewSigner(chainConfig, privateKeyInput)
	if err != nil {
		return "", fmt.Errorf("could not import private key: %v", err)
	}

	publicKey, err := signer.PublicKey()
	if err != nil {
		return "", fmt.Errorf("could not create public key: %v", err)
	}

	addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig)
	if err != nil {
		return "", fmt.Errorf("could not create address builder: %v", err)
	}

	from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("could not derive address: %v", err)
	}
	logrus.WithField("address", from).Info("sending from")

	input, err := client.FetchLegacyTxInput(context.Background(), from, xc.Address(toWalletAddress))
	if err != nil {
		return "", fmt.Errorf("could not fetch transfer input: %v", err)
	}

	if inputWithPublicKey, ok := input.(xc.TxInputWithPublicKey); ok {
		inputWithPublicKey.SetPublicKey(publicKey)
		logrus.WithField("public_key", hex.EncodeToString(publicKey)).Debug("added public key to transfer input")
	}

	if inputWithAmount, ok := input.(xc.TxInputWithAmount); ok {
		inputWithAmount.SetAmount(amountBlockchain)
	}

	if memo != "" {
		if txInputWithMemo, ok := input.(xc.TxInputWithMemo); ok {
			txInputWithMemo.SetMemo(memo)
		} else {
			return "", fmt.Errorf("cannot set memo; chain driver currently does not support memos")
		}
	}
	bz, _ := json.Marshal(input)
	logrus.WithField("input", string(bz)).Debug("transfer input")

	// create tx
	// (no network, no private key needed)
	builder, err := xcFactory.NewTxBuilder(AssetConfig(chainConfig, contract, decimals))
	if err != nil {
		return "", fmt.Errorf("could not load tx-builder: %v", err)
	}

	tx, err := builder.NewTransfer(from, xc.Address(toWalletAddress), amountBlockchain, input)
	if err != nil {
		return "", fmt.Errorf("could not build transfer: %v", err)
	}

	sighashes, err := tx.Sighashes()
	if err != nil {
		return "", fmt.Errorf("could not create payloads to sign: %v", err)
	}

	// sign
	signatures := []xc.TxSignature{}
	for _, sighash := range sighashes {
		// sign the tx sighash(es)
		signature, err := signer.Sign(sighash)
		if err != nil {
			panic(err)
		}
		signatures = append(signatures, signature)
	}

	// complete the tx by adding its signature
	// (no network, no private key needed)
	err = tx.AddSignatures(signatures...)
	if err != nil {
		return "", fmt.Errorf("could not add signature(s): %v", err)
	}

	// submit the tx, wait a bit, fetch the tx info
	// (network needed)
	err = client.SubmitTx(context.Background(), tx)
	if err != nil {
		return "", fmt.Errorf("could not broadcast: %v", err)
	}
	logrus.WithField("hash", tx.Hash()).Info("submitted tx")
	start := time.Now()
	for time.Since(start) < timeout {
		time.Sleep(5 * time.Second)
		info, err := client.FetchTxInfo(context.Background(), tx.Hash())
		if err != nil {
			logrus.WithField("hash", tx.Hash()).WithError(err).Info("could not find tx on chain yet, trying again...")
			continue
		}
		bz, _ := json.MarshalIndent(info, "", "  ")

		return string(bz), nil
	}

	return "", fmt.Errorf("could not find transaction that we submitted by hash %s", tx.Hash())
}

func DeriveAddress(xcFactory *factory.Factory, chainConfig *xc.ChainConfig, privateKey string) (string, error) {
	signer, err := xcFactory.NewSigner(chainConfig, privateKey)
	if err != nil {
		return "", fmt.Errorf("could not import private key: %v", err)
	}

	publicKey, err := signer.PublicKey()
	if err != nil {
		return "", fmt.Errorf("could not create public key: %v", err)
	}

	addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig)
	if err != nil {
		return "", fmt.Errorf("could not create address builder: %v", err)
	}

	from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("could not derive address: %v", err)
	}

	return string(from), nil
}

func AssetConfig(chain *xc.ChainConfig, contractMaybe string, decimals int32) xc.ITask {
	if contractMaybe != "" {
		token := xc.TokenAssetConfig{
			Contract:    contractMaybe,
			Chain:       chain.Chain,
			ChainConfig: chain,
			Decimals:    decimals,
		}
		return &token
	} else {
		return chain
	}
}
