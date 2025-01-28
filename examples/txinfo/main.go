package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory"
)

func TxInfo(ctx context.Context, xc *factory.Factory, nativeAsset crosschain.NativeAsset, txHash string) {
	// get asset model, including config data
	// asset is used to create client, builder, signer, etc.
	asset, ok := xc.GetChain(nativeAsset)
	if !ok {
		panic("unsupported asset: " + nativeAsset)
	}

	// fetch tx info
	client, _ := xc.NewClient(asset)
	info, err := client.FetchTxInfo(ctx, crosschain.TxHash(txHash))
	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", info)
	for _, dst := range info.Movements {
		fmt.Printf("transfer: %+v\n", dst)
	}
}

func main() {
	// initialize crosschain
	config.ConfigureLogger("debug")
	xc := factory.NewDefaultFactory()
	ctx := context.Background()
	if len(os.Args) != 3 {
		fmt.Printf(`usage: ./main <chain> <tx-hash>

where <chain> is a chain indicator like ETH, APTOS, SOL, etc.
`)
		return
	}
	chain := os.Args[1]
	txHash := os.Args[2]
	// TxInfo(ctx, xc, "INJ", "a8522e7c84d462011265cb6038b5a52f3028d4acff14d9565b3cbec0f4a2f800")
	// TxInfo(ctx, xc, "SOL", "3mDyJibiCCXEgfyYcW21Cu9o89qZsgpr2J3n3fpF2EzoT9psDmfyFq3Lv5MxbvvrjkLVnk2KC1TAe7vSTjFcyGHV")
	TxInfo(ctx, xc, crosschain.NativeAsset(chain), txHash)
}
