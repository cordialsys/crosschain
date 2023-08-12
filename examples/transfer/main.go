package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jumpcrypto/crosschain"
	"github.com/jumpcrypto/crosschain/factory"
)

func main() {
	// initialize crosschain
	xc := factory.NewDefaultFactory()
	ctx := context.Background()
	assets := xc.GetAllAssets()
	return
	for _, asset := range assets {
		if n, ok := asset.(*crosschain.NativeAssetConfig); ok {

			fmt.Printf(`
	{
		Asset:         string(xc.%s),
		Driver:       string(xc.%s),
		Net:           "%s",
		URL:           "%s",
		ChainName:     "%s",
	 	ExplorerURL:   "%s",`,
				n.Asset, strings.ReplaceAll(n.Driver, "-", ""), n.Net,
				n.URL, n.ChainName, n.ExplorerURL,
			)
			// 	:    "%s",
			// 	:   "%s",
			// 	: "%s",
			// 	:      %d,
			// 	:       %d,
			// 	:     "%s",
			// 	: "%s",
			// 	: %.2f,
			// 	: %d,
			// 	: %t,
			// 	: "%s",
			// 	: "%s",
			// 	: "%s",
			// 	: %d,
			// 	: %f,
			// },
			if n.Auth != "" {
				fmt.Printf("\n	Auth: \"%s\",", n.Auth)
			}
			if n.IndexerUrl != "" {
				fmt.Printf("\n"+`	IndexerUrl: "%s",`, n.IndexerUrl)
			}
			if n.IndexerType != "" {
				fmt.Printf("\n"+`	IndexerType: "%s",`, n.IndexerType)
			}
			if n.PollingPeriod != "" {
				fmt.Printf("\n"+`	PollingPeriod: "%s",`, n.PollingPeriod)
			}
			if n.Decimals != 0 {
				fmt.Printf("\n"+`	Decimals: %d,`, n.Decimals)
			}
			if n.ChainID != 0 {
				fmt.Printf("\n"+`	ChainID: %d,`, n.ChainID)
			}
			if n.Provider != "" {
				fmt.Printf("\n"+`	Provider: "%s",`, n.Provider)
			}
			if n.ChainGasMultiplier != 0 {
				fmt.Printf("\n"+`	ChainGasMultiplier: %.2f,`, n.ChainGasMultiplier)
			}
			if n.ChainGasPriceDefault != 0 {
				fmt.Printf("\n"+`	ChainGasPriceDefault: %.2f,`, n.ChainGasPriceDefault)
			}
			if n.ChainGasTip != 0 {
				fmt.Printf("\n"+`	ChainGasTip: %d,`, n.ChainGasTip)
			}
			if n.ChainTransferTax != 0 {
				fmt.Printf("\n"+`	ChainTransferTax: %.2f,`, n.ChainTransferTax)
			}
			if n.NoGasFees {
				fmt.Printf("\n"+`	NoGasFees: %t,`, n.NoGasFees)
			}
			if n.GasCoin != "" {
				fmt.Printf("\n"+`	GasCoin: "%s",`, n.GasCoin)
			}
			if n.ChainIDStr != "" {
				fmt.Printf("\n"+`	ChainIDStr: "%s",`, n.ChainIDStr)
			}
			if n.ChainPrefix != "" {
				fmt.Printf("\n"+`	ChainPrefix: "%s",`, n.ChainPrefix)
			}
			if n.ChainCoin != "" {
				fmt.Printf("\n"+`	ChainCoin: "%s",`, n.ChainCoin)
			}
			if n.ChainCoinHDPath != 0 {
				fmt.Printf("\n"+`	ChainCoinHDPath: %d,`, n.ChainCoinHDPath)
			}
			fmt.Printf("\n" + `	},`)

			// n.IndexerUrl, n.IndexerType, n.PollingPeriod, n.Decimals, n.ChainID,
			// n.Provider, n.Auth, n.ChainGasMultiplier, n.ChainGasTip,
			// n.NoGasFees, n.ChainIDStr, n.ChainPrefix, n.ChainCoin, n.ChainCoinHDPath, n.ChainGasPriceDefault,
		}
	}
	return

	// get asset model, including config data
	// asset is used to create client, builder, signer, etc.
	asset, err := xc.GetAssetConfig("", "MATIC")
	if err != nil {
		panic("unsupported asset")
	}

	// set your own private key and address
	// you can get them, for example, from your Phantom wallet
	privateKeyInput := os.Getenv("PRIVATE_KEY")
	if privateKeyInput == "" {
		log.Fatalln("must set env PRIVATE_KEY")
	}

	fromPrivateKey := xc.MustPrivateKey(asset, privateKeyInput)

	signer, _ := xc.NewSigner(asset)
	publicKey, err := signer.PublicKey(fromPrivateKey)
	if err != nil {
		panic("could not create public key: " + err.Error())
	}

	addressBuilder, err := xc.NewAddressBuilder(asset)
	if err != nil {
		panic("could not create address builder: " + err.Error())
	}

	from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
	if err != nil {
		panic("could create from address: " + err.Error())
	}

	to := xc.MustAddress(asset, "0x3ad57b83B2E3dC5648F32e98e386935A9B10bb9F")
	amount := xc.MustAmountBlockchain(asset, "0.001")

	// to create a tx, we typically need some input from the blockchain
	// e.g., nonce for Ethereum, recent block for Solana, gas data, ...
	// (network needed)
	client, _ := xc.NewClient(asset)

	input, err := client.FetchTxInput(ctx, from, to)
	if err != nil {
		panic(err)
	}
	if inputWithPublicKey, ok := input.(crosschain.TxInputWithPublicKey); ok {
		fromPublicKeyStr := base64.StdEncoding.EncodeToString(publicKey)
		inputWithPublicKey.SetPublicKeyFromStr(fromPublicKeyStr)
	}
	fmt.Printf("%+v\n", input)

	// create tx
	// (no network, no private key needed)
	builder, _ := xc.NewTxBuilder(asset)
	tx, err := builder.NewTransfer(from, to, amount, input)
	if err != nil {
		panic(err)
	}
	sighashes, err := tx.Sighashes()
	if err != nil {
		panic(err)
	}
	sighash := sighashes[0]
	fmt.Printf("%+v\n", tx)
	fmt.Printf("signing: %x\n", sighash)

	// sign the tx sighash
	signature, err := signer.Sign(fromPrivateKey, sighash)
	if err != nil {
		panic(err)
	}
	fmt.Printf("signature: %x\n", signature)

	// complete the tx by adding its signature
	// (no network, no private key needed)
	err = tx.AddSignatures(signature)
	if err != nil {
		panic(err)
	}

	// submit the tx, wait a bit, fetch the tx info
	// (network needed)
	fmt.Printf("tx id: %s\n", tx.Hash())
	err = client.SubmitTx(ctx, tx)
	if err != nil {
		panic(err)
	}
	fmt.Println("Zzz...")
	time.Sleep(60 * time.Second)
	info, err := client.FetchTxInfo(ctx, tx.Hash())
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", info)
}
