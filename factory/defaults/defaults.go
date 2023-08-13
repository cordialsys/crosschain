package defaults

import (
	"strings"

	xc "github.com/jumpcrypto/crosschain"
	factoryconfig "github.com/jumpcrypto/crosschain/factory/config"
	"github.com/jumpcrypto/crosschain/factory/defaults/chains"
	"github.com/jumpcrypto/crosschain/factory/defaults/pipelines"
	"github.com/jumpcrypto/crosschain/factory/defaults/tasks"
	"github.com/jumpcrypto/crosschain/factory/defaults/tokens"
	"github.com/sirupsen/logrus"
)

var mainnetChainMap = map[string]*xc.NativeAssetConfig{}
var testnetChainMap = map[string]*xc.NativeAssetConfig{}

func init() {
	for _, chain := range chains.Mainnet {
		asset := strings.ToLower(chain.Asset)
		if _, ok := mainnetChainMap[asset]; ok {
			logrus.Warnf("multiple mainnet configuration entries for %s", asset)
		}
		mainnetChainMap[asset] = chain
	}
	for _, chain := range chains.Testnet {
		asset := strings.ToLower(chain.Asset)
		if _, ok := testnetChainMap[asset]; ok {
			logrus.Warnf("multiple testnet configuration entries for %s", asset)
		}
		testnetChainMap[asset] = chain
	}
}

var Mainnet = factoryconfig.Config{
	Network:   "mainnet",
	Chains:    mainnetChainMap,
	Tokens:    tokens.Mainnet,
	Pipelines: pipelines.Mainnet,
	Tasks:     tasks.Mainnet,
}

var Testnet = factoryconfig.Config{
	Network:   "testnet",
	Chains:    testnetChainMap,
	Tokens:    tokens.Testnet,
	Pipelines: pipelines.Testnet,
	Tasks:     tasks.Testnet,
}
