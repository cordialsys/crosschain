package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory"
)

func main() {
	// initialize crosschain
	xc := factory.NewDefaultFactory()
	ctx := context.Background()

	// get asset model, including config data
	// asset is used to create client, builder, signer, etc.
	asset, err := xc.GetAssetConfig("", crosschain.TRX)
	if err != nil {
		panic("unsupported asset: " + err.Error())
	}

	// asset, err = xc.GetAssetConfig("USDT", crosschain.TRX)
	// if err != nil {
	// 	panic("unsupported asset: " + err.Error())
	// }

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
	fmt.Println("sending from: ", from)
	to := xc.MustAddress(asset, "TFzvbsVRAYDtPGc4KdFNUZophTDxfKD3G1")
	amount := xc.MustAmountBlockchain(asset, "0.5")
	fmt.Println(amount)
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
	if inputWithAmount, ok := input.(crosschain.TxInputWithAmount); ok {
		inputWithAmount.SetAmount(amount)
	}

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

	// sign the tx sighash
	signature, err := signer.Sign(fromPrivateKey, sighash)
	if err != nil {
		panic(err)
	}

	// complete the tx by adding its signature
	// (no network, no private key needed)
	err = tx.AddSignatures(signature)
	if err != nil {
		panic(err)
	}

	// submit the tx, wait a bit, fetch the tx info
	// (network needed)
	err = client.SubmitTx(ctx, tx)
	if err != nil {
		panic(err)
	}
	fmt.Println("Zzz...")
	time.Sleep(10 * time.Second)
	info, err := client.FetchTxInfo(ctx, tx.Hash())
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", info)
}
