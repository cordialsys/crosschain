package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
)

func tryTon() error {
	client := liteclient.NewConnectionPool()
	configUrl := "https://ton-blockchain.github.io/testnet-global.config.json"
	client.AddConnectionsFromConfigUrl(context.Background(), configUrl)
	api := ton.NewAPIClient(client)
	_ = api
	fmt.Println("querying ton")

	// wallet.FromPrivateKey()
	keyseedhex := os.Getenv("PRIVATE_KEY")
	keyseed, _ := hex.DecodeString(keyseedhex)
	key := ed25519.NewKeyFromSeed(keyseed)

	w, err := wallet.FromPrivateKey(api, key, wallet.V3)
	if err != nil {
		return err
	}

	s := w.Address().String()
	fmt.Println(s)

	toAddr := address.MustParseAddr("0QA0jYhujQbxi5MGxEI14bAxP06bVQMFniyg-GL2cI2h45CW")

	tf, err := w.BuildTransfer(toAddr, tlb.MustFromTON("0.003"), false, "")
	if err != nil {
		return err
	}
	c, _ := tlb.ToCell(tf.InternalMessage)
	fmt.Println("transfer message: ", hex.EncodeToString(c.ToBOCWithFlags(false)))

	err = w.Send(context.Background(), tf, true)

	return err
}

func main() {
	err := tryTon()
	if err != nil {
		log.Fatalf("%v", err)
	}
}
