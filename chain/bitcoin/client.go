package bitcoin

import (
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/blockchair"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/native"
	"github.com/cordialsys/crosschain/client"
)

func NewClient(cfgI xc.ITask) (client.Client, error) {

	switch cfgI.GetChain().Provider {
	case "native":
		return native.NewNativeClient(cfgI)
	case "blockchain":
		return blockchair.NewBlockchairClient(cfgI)
	case "blockbook":
		// TODO
		panic("blockbook")
	default:
		if strings.Contains(cfgI.GetChain().URL, "blockchair") {
			return blockchair.NewBlockchairClient(cfgI)
		}

		// TODO return blockbook
		// panic("blockbook")
		return blockchair.NewBlockchairClient(cfgI)
	}
}
