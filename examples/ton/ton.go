package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
)

func tryTon() error {
	client := liteclient.NewConnectionPool()
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

	// balance, err := w.GetBalance(context.Background(), nil)
	// if err != nil {
	// 	panic(err)
	// }

	// if balance.Nano().Uint64() >= 3000000 {
	// 	addr := address.MustParseAddr("EQCD39VS5jcptHL8vMjEXrzGaRcCVYto7HUn4bpAOg8xqB2N")
	// 	err = w.Transfer(context.Background(), addr, tlb.MustFromTON("0.003"), "Hey bro, happy birthday!")
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// }

	return nil
}

func main() {
	err := tryTon()
	if err != nil {
		log.Fatalf("%v", err)
	}
}
