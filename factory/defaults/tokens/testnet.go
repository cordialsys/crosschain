package tokens

import xc "github.com/jumpcrypto/crosschain"

func init() {
	for _, chain := range Testnet {
		if chain.Net == "" {
			chain.Net = "testnet"
		}
	}
}

var Testnet = []*xc.TokenAssetConfig{
	{
		Asset:    "DAI",
		Chain:    string(xc.ETH),
		Decimals: 18,
		Contract: "0xc2118d4d90b274016cb7a54c03ef52e6c537d957",
	},
	{
		Asset:    "USDC",
		Chain:    string(xc.ETH),
		Decimals: 6,
		Contract: "0x07865c6e87b9f70255377e024ace6630c1eaa37f",
	},
	{
		Asset:    "WETH",
		Chain:    string(xc.ETH),
		Decimals: 18,
		Contract: "0xB4FBF271143F4FBf7B91A5ded31805e42b2208d6",
	},
	{
		Asset:    "WETH",
		Chain:    string(xc.MATIC),
		Decimals: 18,
		Contract: "0xc6735cc74553Cc2caeB9F5e1Ea0A4dAe12ef4632",
	},

	{
		Asset:    "USDC",
		Chain:    string(xc.SOL),
		Decimals: 6,
		Contract: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
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
	// Add some unique token mechanisms on cosmos
	{
		Asset:    "USDC",
		Chain:    string(xc.INJ),
		Decimals: 6,
		Contract: "factory/inj17vytdwqczqz72j65saukplrktd4gyfme5agf6c/usdc",
	},
	{
		Asset:    "USDT",
		Chain:    string(xc.INJ),
		Decimals: 6,
		Contract: "peggy0x87aB3B4C8661e07D6372361211B96ed4Dc36B1B5",
	},
}
