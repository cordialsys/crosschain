package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory"
)

func main() {
	// initialize crosschain
	config.ConfigureLogger("debug")
	xc := factory.NewDefaultFactory()
	ctx := context.Background()
	if len(os.Args) != 3 {
		log.Fatalf("usage: ./main <chain> <address>")
	}

	// get asset model, including config data
	// asset is used to create client, builder, signer, etc.
	asset, err := xc.GetAssetConfig("", crosschain.NativeAsset(os.Args[1]))
	if err != nil {
		panic("unsupported asset: " + err.Error())
	}

	from := xc.MustAddress(asset, os.Args[2])
	amount := xc.MustAmountBlockchain(asset, "0.5")
	fmt.Println(amount)
	// to create a tx, we typically need some input from the blockchain
	// e.g., nonce for Ethereum, recent block for Solana, gas data, ...
	// (network needed)
	client, _ := xc.NewClient(asset)

	input, err := client.FetchTxInput(ctx, from, "")
	if err != nil {
		panic(err)
	}
	inputBz, _ := json.MarshalIndent(input, "", "  ")
	fmt.Println("input: ", string(inputBz))
}
