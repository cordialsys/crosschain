package defaults

import (
	factoryconfig "github.com/jumpcrypto/crosschain/factory/config"
	"github.com/jumpcrypto/crosschain/factory/defaults/chains"
	"github.com/jumpcrypto/crosschain/factory/defaults/pipelines"
	"github.com/jumpcrypto/crosschain/factory/defaults/tasks"
	"github.com/jumpcrypto/crosschain/factory/defaults/tokens"
)

var Mainnet = factoryconfig.Config{
	Network:   "mainnet",
	Chains:    chains.Mainnet,
	Tokens:    tokens.Mainnet,
	Pipelines: pipelines.Mainnet,
	Tasks:     tasks.Mainnet,
}

var Testnet = factoryconfig.Config{
	Network:   "testnet",
	Chains:    chains.Testnet,
	Tokens:    tokens.Testnet,
	Pipelines: pipelines.Testnet,
	Tasks:     tasks.Testnet,
}
