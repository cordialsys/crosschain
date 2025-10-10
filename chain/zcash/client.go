package zcash

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	"github.com/cordialsys/crosschain/chain/zcash/address"
	"github.com/cordialsys/crosschain/client"
)

func NewClient(cfgI xc.ITask) (client.Client, error) {
	cli, err := bitcoin.NewBitcoinClient(cfgI)
	if err != nil {
		return cli, err
	}
	return cli.WithAddressDecoder(&address.ZcashAddressDecoder{}).(bitcoin.BtcClient), nil
}
