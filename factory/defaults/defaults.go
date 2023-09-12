package defaults

import (
	"strings"

	xc "github.com/cordialsys/crosschain"
	factoryconfig "github.com/cordialsys/crosschain/factory/config"
	"github.com/cordialsys/crosschain/factory/defaults/chains"
	"github.com/cordialsys/crosschain/factory/defaults/pipelines"
	"github.com/cordialsys/crosschain/factory/defaults/tasks"
	"github.com/cordialsys/crosschain/factory/defaults/tokens"
	"github.com/sirupsen/logrus"
)

type HasID interface {
	ID() xc.AssetID
}

func listToMap[T HasID](list []T) map[string]T {
	toMap := map[string]T{}
	for _, item := range list {
		asset := strings.ToLower(string(item.ID()))
		if _, ok := toMap[asset]; ok {
			logrus.Warnf("multiple entries for %s (%T)", asset, item)
		}
		toMap[asset] = item
	}
	return toMap
}

var mainnetTaskMap = listToMap(tasks.Mainnet)
var testnetTaskMap = listToMap(tasks.Testnet)

var mainnetPipelineMap = listToMap(pipelines.Mainnet)
var testnetPipelineMap = listToMap(pipelines.Testnet)

var Mainnet = factoryconfig.Config{
	Network:   "mainnet",
	Chains:    chains.Mainnet,
	Tokens:    tokens.Mainnet,
	Pipelines: mainnetPipelineMap,
	Tasks:     mainnetTaskMap,
}

var Testnet = factoryconfig.Config{
	Network:   "testnet",
	Chains:    chains.Testnet,
	Tokens:    tokens.Testnet,
	Pipelines: testnetPipelineMap,
	Tasks:     testnetTaskMap,
}
