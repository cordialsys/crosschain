package defaults

import (
	factoryconfig "github.com/cordialsys/crosschain/factory/config"
	"github.com/cordialsys/crosschain/factory/defaults/chains"
)

var Mainnet = factoryconfig.Config{
	Network: "mainnet",
	Chains:  chains.Mainnet,
}

var Testnet = factoryconfig.Config{
	Network: "testnet",
	Chains:  chains.Testnet,
}
