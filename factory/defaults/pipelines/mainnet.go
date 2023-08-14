package pipelines

import xc "github.com/jumpcrypto/crosschain"

var Mainnet = []*xc.PipelineConfig{
	{
		Name: "wormhole-transfer",
		Allow: []string{
			"USDC.ETH -> USDCet.MATIC",
			"USDT.ETH -> USDTet.MATIC",
			"WETH.ETH -> WETH.MATIC",
			"WBTC.ETH -> WBTC.MATIC",
			"LM_SNA.ETH -> SOL.MATIC",
			"AVAX.ETH -> AVAX.MATIC",
			"YFI.ETH -> YFIet.MATIC",
			"ONEINCH.ETH -> ONEINCHet.MATIC",
			"BIT.ETH -> BITet.MATIC",
			"COMP.ETH -> COMPet.MATIC",
			"SUSHI.ETH -> SUSHIet.MATIC",
			"DAI.ETH -> DAIet.MATIC",
			"wAVAX.AVAX -> AVAX.MATIC",
			"wFTM.FTM -> FTM.MATIC",
		},
		Tasks: []string{
			"wormhole-approve",
			"wormhole-transfer",
		},
	},
}
