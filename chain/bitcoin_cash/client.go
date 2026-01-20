package bitcoin_cash

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	"github.com/cordialsys/crosschain/client"
)

func NewClient(cfgI *xc.ChainConfig) (client.Client, error) {
	cli, err := bitcoin.NewBitcoinClient(cfgI)
	if err != nil {
		return cli, err
	}
	return cli.WithAddressDecoder(&BchAddressDecoder{}).(bitcoin.BtcClient), nil
}
