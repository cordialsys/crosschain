package main

import (
	"fmt"

	"github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/config"
	"github.com/cordialsys/crosschain/factory/defaults/chains"
	"gopkg.in/yaml.v3"
)

func main() {
	chains := chains.Mainnet
	// tokens := tokens.Testnet
	cfg := &config.Config{
		Chains: make(map[string]*crosschain.ChainConfig),
	}
	for _, chain := range chains {
		// cfg.Chains[chain.Asset] = chain
		_ = chain
	}

	bz, err := yaml.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bz))

}
