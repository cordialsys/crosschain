package tasks

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/defaults/tasks/contracts"
	"github.com/cordialsys/crosschain/factory/defaults/tasks/contracts/mainnet"
	"github.com/cordialsys/crosschain/factory/defaults/tasks/contracts/testnet"
)

var Mainnet = GenerateTasks(
	mainnet.CoinbaseEvmSenderContract,
	mainnet.WormholeTokenBridge,
	mainnet.WormholeChainMapping,
)

var Testnet = GenerateTasks(
	testnet.CoinbaseEvmSenderContract,
	testnet.WormholeTokenBridge,
	testnet.WormholeChainMapping,
)

func GenerateTasks(
	conbaseMultisendContract string,
	wormholeTokenBridge contracts.WormholeTokenContractMapping,
	wormholeChains contracts.WormholeXcChainMapping,
) []*xc.TaskConfig {
	return []*xc.TaskConfig{
		{
			Name:  "sol-wrap",
			Code:  "WrapTx",
			Chain: string(xc.SOL),
			Allow: []string{
				"SOL -> WSOL.SOL",
			},
		},
		{
			Name:  "sol-unwrap",
			Code:  "UnwrapEverythingTx",
			Chain: string(xc.SOL),
			Allow: []string{
				"WSOL.SOL -> SOL",
			},
		},
		{
			Name:  "eth-wrap",
			Chain: string(xc.ETH),
			Allow: []string{
				"ETH -> WETH.ETH",
				"ArbETH -> WETH.ArbETH",
				"MATIC -> WMATIC.MATIC",
				"BNB -> WBNB.BNB",
			},
			Operations: []xc.TaskConfigOperation{
				{
					Function:  "deposit",
					Contract:  "dst_asset",
					Signature: "d0e30db0",
					Payable:   true,
				},
			},
		},
		{
			Name:  "eth-unwrap",
			Chain: string(xc.ETH),
			Allow: []string{
				"WETH.ETH -> ETH",
				"WETH.ArbETH -> ETH",
				"WMATIC.MATIC-> MATIC",
				"WBNB.BNB -> BNB",
			},
			Operations: []xc.TaskConfigOperation{
				{
					Function:  "withdraw",
					Signature: "2e1a7d4d",
					Payable:   true,
					Params: []xc.TaskConfigOperationParam{
						{
							Name: "qty",
							Type: "uint256",
							Bind: "amount",
						},
					},
				},
			},
		},

		{
			Name: "coinbase-multisend-eth",
			Code: "MultisendTransferTx",
			Operations: []xc.TaskConfigOperation{
				{
					Function:  "multisendETH",
					Signature: "1a1da075",
					Contract:  conbaseMultisendContract,
					Params: []xc.TaskConfigOperationParam{
						{
							Name: "ignored1",
							Type: "uint256",
						},
						{
							Name: "ignored2",
							Type: "uint256",
						},
						{
							Name: "tx",
							// array is not yet fully implemented
							Type: "array",
							Bind: "destinations",
							Fields: []xc.TaskConfigOperationParam{
								{
									Bind: "to",
									Type: "address",
								},
								{
									Bind: "amount",
									Type: "uint256",
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "coinbase-multisend-erc20",
			Code: "MultisendTransferTx",
			Operations: []xc.TaskConfigOperation{
				{
					Function:  "multisendERC20",
					Signature: "ca350aa6",
					Contract:  conbaseMultisendContract,
					Params: []xc.TaskConfigOperationParam{
						{
							Name: "ignored1",
							Type: "uint256",
						},
						{
							Name: "ignored2",
							Type: "uint256",
						},
						{
							Name: "tx",
							// array is not yet fully implemented
							Type: "array",
							Bind: "destinations",
							Fields: []xc.TaskConfigOperationParam{
								{
									Bind: "to",
									Type: "address",
								},
								{
									Bind: "amount",
									Type: "uint256",
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "wormhole-approve",
			Operations: []xc.TaskConfigOperation{
				{
					Function:  "approve",
					Signature: "095ea7b3",
					Params: []xc.TaskConfigOperationParam{
						{
							Name: "spender",
							Type: "address",
							// we need to know the token bridge contract address to send approvals to
							Value: wormholeTokenBridge,
						},
						{
							Name: "amount",
							Type: "uint256",
							Bind: "amount",
						},
					},
				},
			},
		},

		{
			Name: "wormhole-transfer",
			Code: "WormholeTransferTx",
			DefaultParams: map[string]interface{}{
				"arbiter_fee_usd": 5,
			},
			Operations: []xc.TaskConfigOperation{
				{
					Function:  "transfer",
					Signature: "0f5287b0",
					// we need to know the token bridge contract address to interactive with per chain
					Contract: wormholeTokenBridge,
					Params: []xc.TaskConfigOperationParam{
						{
							Name: "token",
							Type: "address",
							Bind: "contract",
						},
						{
							Name: "amount",
							Type: "uint256",
							Bind: "amount",
						},
						{
							Name:  "chain",
							Match: "dst_asset",
							Type:  "uint256",
							Value: wormholeChains,
						},
						{
							Name: "recipient",
							Type: "address",
							Bind: "to",
						},
					},
				},
			},
		},

		// EVM functions crosschain natively supports but written as code task
		{
			Name: "evm-transfer",
			Code: "ProxyTransferTx",
			Allow: []string{
				"ETH",
			},
		},
		{
			Name: "evm-transfer-erc20",
			Code: "ProxyTransferTx",
			Allow: []string{
				"*",
			},
		},

		// EVM functions crosschain natively supports but written as pure task
		{
			Name:  "erc20-transfer",
			Chain: "ETH",
			Allow: []string{
				"USDC.ETH",
				"WETH.ETH",
			},
			Operations: []xc.TaskConfigOperation{
				{
					Function:  "transfer",
					Signature: "a9059cbb",
					Params: []xc.TaskConfigOperationParam{
						{
							Name: "addr",
							Type: "address",
							Bind: "to",
						},
						{
							Name: "qty",
							Type: "uint256",
							Bind: "amount",
						},
					},
				},
			},
		},
		{
			Name:  "KLAY Unstake",
			Allow: []string{"KLAY.KLAY"},
			Operations: []xc.TaskConfigOperation{
				{
					Function:  "deposit",
					Contract:  "0x0795aea6948fc1d31809383edc4183b220abd71f", // mainnet
					Signature: "238be93f",
					Payable:   false,
					Params: []xc.TaskConfigOperationParam{
						{
							Name: "to",
							Type: "address",
							Bind: "from",
						},
						{
							Name: "klay",
							Type: "uint256",
							Bind: "amount",
						},
					},
				},
			},
		},
	}

}
