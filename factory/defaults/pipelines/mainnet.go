package pipelines

import xc "github.com/jumpcrypto/crosschain"

var Mainnet = []*xc.PipelineConfig{
	{
		ID: "wormhole-transfer",
		Allow: []string{
			"WETH.ETH -> WETH.MATIC",
			"WETH.ETH -> WETH.SOL",
			"WETH.MATIC -> WETH.SOL",
		},
		Tasks: []string{
			"wormhole-approve",
			"wormhole-transfer",
		},
	},
}
