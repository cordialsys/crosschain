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
	if len(os.Args) < 4 {
		log.Fatalf("usage: ./main <chain> <from-address> <to-address> [token-contract]")
	}

	chain := os.Args[1]
	fromStr := os.Args[2]
	to := os.Args[3]
	contract := ""
	if len(os.Args) > 4 {
		contract = os.Args[4]
	}

	// get asset model, including config data
	// asset is used to create client, builder, signer, etc.
	asset, err := xc.GetAssetConfigByContract(contract, crosschain.NativeAsset(chain))
	if err != nil {
		panic("unsupported asset: " + err.Error())
	}

	from := xc.MustAddress(asset, fromStr)
	amount := xc.MustAmountBlockchain(asset, "0.5")
	fmt.Println(amount)
	// to create a tx, we typically need some input from the blockchain
	// e.g., nonce for Ethereum, recent block for Solana, gas data, ...
	// (network needed)
	client, _ := xc.NewClient(asset)

	input, err := client.FetchLegacyTxInput(ctx, from, crosschain.Address(to))
	if err != nil {
		panic(err)
	}
	inputBz, _ := json.MarshalIndent(input, "", "  ")
	fmt.Println("input: ", string(inputBz))
}
