package tokens

import xc "github.com/jumpcrypto/crosschain"

func init() {
	for _, chain := range Mainnet {
		if chain.Net == "" {
			chain.Net = "mainnet"
		}
	}
}

// There are too many tokens than we can list here but will keep some defaults
var Mainnet = []*xc.TokenAssetConfig{
	{
		Asset:    "DAI",
		Chain:    string(xc.ETH),
		Decimals: 18,
		Contract: "0x6b175474e89094c44da98b954eedeac495271d0f",
	},
	{
		Asset:    "USDC",
		Chain:    string(xc.ETH),
		Decimals: 6,
		Contract: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
	},
	{
		Asset:    "WETH",
		Chain:    string(xc.ETH),
		Decimals: 18,
		Contract: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
	},
	{
		Asset:    "WETH",
		Chain:    string(xc.MATIC),
		Decimals: 18,
		Contract: "0x7ceB23fD6bC0adD59E62ac25578270cFf1b9f619",
	},

	{
		Asset:    "USDC",
		Chain:    string(xc.SOL),
		Decimals: 6,
		Contract: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
	},
	{
		Asset:    "WSOL",
		Chain:    string(xc.SOL),
		Decimals: 9,
		Contract: "So11111111111111111111111111111111111111112",
	},

	{
		Asset:    "USTC",
		Chain:    string(xc.LUNC),
		Decimals: 6,
		Contract: "uusd",
	},
}
