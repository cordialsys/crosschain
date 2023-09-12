package testnet

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/defaults/tasks/contracts/mainnet"
)

// TESTNET

// Chain -> Contract address
var WormholeTokenBridge = map[string]string{
	string(xc.ETH):  "0x3ee18B2214AFF97000D974cf647E7C347E8fa585",
	string(xc.FTM):  "0x7C9Fc5741288cDFdD83CeB07f3ea7e22618D79D2",
	string(xc.AVAX): "0x0e082F06FF657D94310cB8cE8B0D9a04541d8052",
}

// Same as mainnet
var WormholeChainMapping = mainnet.WormholeChainMapping

// doesn't exist
var CoinbaseEvmSenderContract = ""
