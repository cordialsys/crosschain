package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	"github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
)

func testbitcoin(addressRaw string) error {
	params, err := params.GetParams(crosschain.NewChainConfig(crosschain.BTC).WithNet("testnet"))
	if err != nil {
		return err
	}
	addr, err := btcutil.DecodeAddress(
		addressRaw,
		params,
	)
	if err != nil {
		return err
	}
	script, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return err
	}
	fmt.Println("script: ", hex.EncodeToString(script))
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("usage: %s <address>\n", os.Args[0])
		return
	}
	err := testbitcoin(os.Args[1])
	if err != nil {
		panic(err)
	}
}
