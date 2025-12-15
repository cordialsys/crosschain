package bitcoin

import (
	"strings"

	log "github.com/sirupsen/logrus"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/address"
	bitcoinclient "github.com/cordialsys/crosschain/chain/bitcoin/client"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/blockchair"
	"github.com/cordialsys/crosschain/client"
)

type BitcoinClient string

var Native BitcoinClient = "native"
var Blockchair BitcoinClient = "blockchair"
var Blockbook BitcoinClient = "blockbook"
var FullBlockbook BitcoinClient = "full-blockbook"
var JsonRpc BitcoinClient = "json-rpc"
var QuicknodeBlockbook BitcoinClient = "quicknode-blockbook"

type BtcClient interface {
	client.Client
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
	log := log.WithField("chain", cfgI.GetChain().Chain)
	if strings.Contains(cfgI.GetChain().URL, "api.blockchair.com") {
		log.Debug("using blockchair client")
		return blockchair.NewBlockchairClient(cfgI)
	}

	switch BitcoinClient(cfgI.GetChain().Provider) {
	case Blockchair:
		log.Debug("using blockchair client")
		return blockchair.NewBlockchairClient(cfgI)
	case Blockbook:
		log.Debug("using blockbook rest client")
		return bitcoinclient.NewBlockbookClient(cfgI)
	case QuicknodeBlockbook:
		log.Debug("using quicknode blockbook rpc client")
		return bitcoinclient.NewQuicknodeBlockbookClient(cfgI)
	case JsonRpc:
		log.Debug("using json-rpc client")
		return bitcoinclient.NewJsonRpcClient(cfgI)
	default:
		log.Debug("using default (json-rpc) client")
		return bitcoinclient.NewJsonRpcClient(cfgI)
	}
}
