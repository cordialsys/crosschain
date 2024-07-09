package bitcoin

import (
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/blockbook"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/blockchair"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/native"
	"github.com/cordialsys/crosschain/client"
)

type BitcoinClient string

var Native BitcoinClient = "native"
var Blockchair BitcoinClient = "blockchair"
var Blockbook BitcoinClient = "blockbook"

func NewClient(cfgI xc.ITask) (client.Client, error) {
	if strings.Contains(cfgI.GetChain().URL, "api.blockchair.com") {
		return blockchair.NewBlockchairClient(cfgI)
	}

	switch BitcoinClient(cfgI.GetChain().Provider) {
	case Native:
		return native.NewNativeClient(cfgI)
	case Blockchair:
		return blockchair.NewBlockchairClient(cfgI)
	case Blockbook:
		return blockbook.NewClient(cfgI)
	default:
		return blockbook.NewClient(cfgI)
	}
}
