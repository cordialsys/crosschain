package bitcoin

import (
	log "github.com/sirupsen/logrus"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/address"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/blockbook"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/blockchair"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/native"
	"github.com/cordialsys/crosschain/client"
)

type BitcoinClient string

var Native BitcoinClient = "native"
var Blockchair BitcoinClient = "blockchair"
var Blockbook BitcoinClient = "blockbook"

type BtcClient interface {
	client.FullClient
	address.WithAddressDecoder
}

func NewClient(cfgI xc.ITask) (BtcClient, error) {
	cli, err := NewBitcoinClient(cfgI)
	if err != nil {
		return cli, err
	}
	return cli.WithAddressDecoder(&address.BtcAddressDecoder{}).(BtcClient), nil
}
func NewBitcoinClient(cfgI xc.ITask) (BtcClient, error) {
	if strings.Contains(cfgI.GetChain().URL, "api.blockchair.com") {
		log.Debug("Running blockchair btc client")
		return blockchair.NewBlockchairClient(cfgI)
	}

	switch BitcoinClient(cfgI.GetChain().Provider) {
	case Native:
		log.Debug("Running native btc client")
		return native.NewNativeClient(cfgI)
	case Blockchair:
		log.Debug("Running blockchair btc client")
		return blockchair.NewBlockchairClient(cfgI)
	case Blockbook:
		log.Debug("Running blockbook btc client")
		return blockbook.NewClient(cfgI)
	default:
		log.Debug("Running default (blockbook) btc client")
		return blockbook.NewClient(cfgI)
	}
}
