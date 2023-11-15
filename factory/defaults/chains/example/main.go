package main

import (
	"fmt"

	"github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/config"
	"github.com/cordialsys/crosschain/factory/defaults/chains"
	"github.com/cordialsys/crosschain/factory/defaults/tokens"
	"gopkg.in/yaml.v3"
)

func main() {
	chains := chains.Mainnet
	// chains := chains.Testnet
	tokens := tokens.Mainnet
	// tokens := tokens.Testnet
	cfg := &config.Config{
		Chains: make(map[string]*crosschain.ChainConfig),
		Tokens: make(map[string]*crosschain.TokenAssetConfig),
	}
	for _, chain := range chains {
		// cfg.Chains[chain.Asset] = chain
		_ = chain
	}
	for _, token := range tokens {
		cfg.Tokens[string(token.ID())] = token
	}

	bz, err := yaml.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bz))

}
