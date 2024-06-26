package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory"
)

func main() {
	// initialize crosschain
	config.ConfigureLogger("debug")
	xc := factory.NewDefaultFactory()
	ctx := context.Background()

	// os.Args[]
	if len(os.Args) != 4 && len(os.Args) != 5 {
		log.Fatalf("usage: ./main [asset] <chain> <amount> <destination>")
	}
	assetInput := ""
	chainInput := os.Args[1]
	amountInput := os.Args[2]
	destination := os.Args[3]
	if len(os.Args) > 4 {
		assetInput = os.Args[1]
		chainInput = os.Args[2]
		amountInput = os.Args[3]
		destination = os.Args[4]
	}

	// get asset model, including config data
	// asset is used to create client, builder, signer, etc.
	asset, err := xc.GetAssetConfig(assetInput, crosschain.NativeAsset(chainInput))
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
	to := xc.MustAddress(asset, destination)
	amount := xc.MustAmountBlockchain(asset, amountInput)
	fmt.Println(amount)
	// to create a tx, we typically need some input from the blockchain
	// e.g., nonce for Ethereum, recent block for Solana, gas data, ...
	// (network needed)
	client, _ := xc.NewClient(asset)

	input, err := client.FetchTxInput(ctx, from, to)
	if err != nil {
		panic("could not fetch" + err.Error())
	}
	inputBz, _ := json.MarshalIndent(input, "", "  ")
	fmt.Println(string(inputBz))
	if inputWithPublicKey, ok := input.(crosschain.TxInputWithPublicKey); ok {
		fromPublicKeyStr := base64.StdEncoding.EncodeToString(publicKey)
		inputWithPublicKey.SetPublicKeyFromStr(fromPublicKeyStr)
		fmt.Println("public key is: " + hex.EncodeToString(publicKey))
	}
	if inputWithAmount, ok := input.(crosschain.TxInputWithAmount); ok {
		inputWithAmount.SetAmount(amount)
	}

	// create tx
	// (no network, no private key needed)
	builder, _ := xc.NewTxBuilder(asset)
	tx, err := builder.NewTransfer(from, to, amount, input)
	if err != nil {
		panic("could not build transfer: " + err.Error())
	}
	sighashes, err := tx.Sighashes()
	if err != nil {
		panic(err)
	}
	signatures := []crosschain.TxSignature{}
	for _, sighash := range sighashes {
		// sign the tx sighash(es)
		signature, err := signer.Sign(fromPrivateKey, sighash)
		if err != nil {
			panic(err)
		}
		signatures = append(signatures, signature)
	}

	// complete the tx by adding its signature
	// (no network, no private key needed)
	err = tx.AddSignatures(signatures...)
	if err != nil {
		panic(err)
	}

	// submit the tx, wait a bit, fetch the tx info
	// (network needed)
	err = client.SubmitTx(ctx, tx)
	if err != nil {
		panic(err)
	}
	fmt.Printf("submitted tx with hash %s\n", tx.Hash())
	fmt.Println("Zzz...")
	for i := 0; i < 10; i++ {
		time.Sleep(5 * time.Second)
		info, err := client.FetchLegacyTxInfo(ctx, tx.Hash())
		if err != nil {
			fmt.Printf("could not find tx %s yet, trying again...\n", tx.Hash())
			continue
		}
		fmt.Printf("%+v\n", info)
		return
	}
	panic("could not find submitted transaction by hash: " + tx.Hash())
}
