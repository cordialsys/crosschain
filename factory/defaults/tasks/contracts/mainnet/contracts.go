package mainnet

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/defaults/tasks/contracts"
)

// MAINNET

// Chain -> Contract address
var WormholeTokenBridge = contracts.WormholeTokenContractMapping{
	string(xc.ETH):  "0x98f3c9e6E3fAce36bAAd05FE09d375Ef1464288B",
	string(xc.FTM):  "0x126783A6Cb203a3E35344528B26ca3a0489a1485",
	string(xc.AVAX): "0x54a8e5f9c4CbA08F9943965859F6c34eAF03E26c",
}

var WormholeChainMapping = contracts.WormholeXcChainMapping{
	string(xc.SOL):    1,
	string(xc.ETH):    2,
	string(xc.LUNC):   3,
	string(xc.BNB):    4,
	string(xc.MATIC):  5,
	string(xc.AVAX):   6,
	string(xc.EmROSE): 7,
	// string(xc.ALGO): 8,
	string(xc.AurETH): 9,
	string(xc.FTM):    10,
	string(xc.KAR):    11,
	string(xc.ACA):    12,
	string(xc.KLAY):   13,
	string(xc.CELO):   14,
	// string(xc.NEAR): 15,
	// string(xc.MOON): 16,
	string(xc.LUNA):   18,
	string(xc.INJ):    19,
	string(xc.SUI):    21,
	string(xc.APTOS):  22,
	string(xc.ArbETH): 23,
	string(xc.OptETH): 24,
	string(xc.XPLA):   28,
}

var CoinbaseEvmSenderContract = "0xa9d1e08c7793af67e9d92fe308d5697fb81d3e43"
