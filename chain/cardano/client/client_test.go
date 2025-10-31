package client_test

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/cardano/client"
	"github.com/cordialsys/crosschain/chain/cardano/client/types"
	"github.com/cordialsys/crosschain/chain/cardano/tx"
	"github.com/cordialsys/crosschain/chain/cardano/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	txinfo "github.com/cordialsys/crosschain/client/tx-info"
	"github.com/stretchr/testify/require"
)

func NewTestConfig() *xc.ChainConfig {
	amount, err := xc.NewAmountHumanReadableFromStr("3.0")
	if err != nil {
		panic(err)
	}
	return xc.NewChainConfig(xc.ADA).
		WithNet("preprod").
		WithDecimals(6).
		WithUrl("https://mainnet.cardano.org").
		WithTransactionActiveTime(time.Hour * 2).
		WithGasBudgetDefault(amount)
}

func TestNewClient(t *testing.T) {
	vectors := []struct {
		testName       string
		url            string
		network        string
		expectedClient *client.Client
		err            string
	}{
		{
			testName: "MainnetClient",
			url:      "https://mainnet.cardano.org",
			network:  "mainnet",
			expectedClient: &client.Client{
				Url:     "https://mainnet.cardano.org",
				Network: "mainnet",
			},
		},
		{
			testName: "ImplicitMainnetClient",
			url:      "https://mainnet.cardano.org",
			network:  "",
			expectedClient: &client.Client{
				Url:     "https://mainnet.cardano.org",
				Network: "mainnet",
			},
		},
		{
			testName: "MissingUrl",
			url:      "",
			network:  "mainnet",
			expectedClient: &client.Client{
				Url:     "https://mainnet.cardano.org",
				Network: "mainnet",
			},
			err: "rpc url is empty",
		},
	}

	for _, vector := range vectors {
		t.Run(vector.testName, func(t *testing.T) {
			client, err := client.NewClient(xc.NewChainConfig(xc.ADA).WithUrl(vector.url).WithNet(vector.network))
			if vector.err != "" {
				require.EqualError(t, err, vector.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, vector.expectedClient.Url, client.Url)
			require.Equal(t, vector.expectedClient.Network, client.Network)
		})
	}

}

func TestFetchTxInput(t *testing.T) {
	vectors := []struct {
		name                       string
		addressUtxosResponse       string
		latestBlockResponse        string
		protocolParametersResponse string
		getAccountInfoResponse     string
		expectedInput              xc.TxInput
		err                        bool
	}{
		{
			name:          "NoReplies",
			expectedInput: &tx_input.TxInput{},
			err:           true,
		},
		{
			name:                 "NoLatestBlockResponse",
			addressUtxosResponse: `[{"address":"addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5","tx_hash":"72cfa181469b48402a50c6652d45c789897ae5025bb01f569a7bd01bffd12bc1","tx_index":1,"output_index":1,"amount":[{"unit":"lovelace","quantity":"5333004"}],"block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","data_hash":null,"inline_datum":null,"reference_script_hash":null}]`,
			expectedInput:        &tx_input.TxInput{},
			err:                  true,
		},
		{
			name:                 "NoProtocolParametersResponse",
			addressUtxosResponse: `[{"address":"addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5","tx_hash":"72cfa181469b48402a50c6652d45c789897ae5025bb01f569a7bd01bffd12bc1","tx_index":1,"output_index":1,"amount":[{"unit":"lovelace","quantity":"5333004"}],"block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","data_hash":null,"inline_datum":null,"reference_script_hash":null}]`,
			latestBlockResponse:  `{"time":1746434616,"height":3446505,"hash":"a13ea6bdb4f23caa803274011c0524e4d96eb7cacb4103998d508c257361cd1b","slot":90751416,"epoch":213,"epoch_slot":377016,"slot_leader":"pool1rccstu3l9ty3k0a5cd06fl3szsss9r34dcg5j38fqgq9kvng0tg","size":4,"tx_count":0,"output":null,"fees":null,"block_vrf":"vrf_vk1kkc5ar4jt2fkdcxp5sa0ekxsskfyjgp082xuqwcn7stvwr2dultsuwejcz","op_cert":"bb4f8eb8a07ca955b55a2c917df0475ccc36cf5487e1af0f97f562717d59ba82","op_cert_counter":"7","previous_block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","next_block":null,"confirmations":0}`,
			expectedInput:        &tx_input.TxInput{},
			err:                  true,
		},
		{
			name:                       "ValidInput",
			protocolParametersResponse: `{"min_fee_a":44,"min_fee_b":155381,"min_utxo":"4310", "coins_per_utxo_word":"4310","key_deposit":"200000"}`,
			addressUtxosResponse:       `[{"address":"addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5","tx_hash":"72cfa181469b48402a50c6652d45c789897ae5025bb01f569a7bd01bffd12bc1","tx_index":1,"output_index":1,"amount":[{"unit":"lovelace","quantity":"5333004"}],"block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","data_hash":null,"inline_datum":null,"reference_script_hash":null}]`,
			latestBlockResponse:        `{"time":1746434616,"height":3446505,"hash":"a13ea6bdb4f23caa803274011c0524e4d96eb7cacb4103998d508c257361cd1b","slot":90751416,"epoch":213,"epoch_slot":377016,"slot_leader":"pool1rccstu3l9ty3k0a5cd06fl3szsss9r34dcg5j38fqgq9kvng0tg","size":4,"tx_count":0,"output":null,"fees":null,"block_vrf":"vrf_vk1kkc5ar4jt2fkdcxp5sa0ekxsskfyjgp082xuqwcn7stvwr2dultsuwejcz","op_cert":"bb4f8eb8a07ca955b55a2c917df0475ccc36cf5487e1af0f97f562717d59ba82","op_cert_counter":"7","previous_block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","next_block":null,"confirmations":0}`,
			expectedInput: &tx_input.TxInput{
				ProtocolParams: types.ProtocolParameters{
					FeePerByte:       44,
					FixedFee:         155381,
					MinUtxoValue:     "4310",
					CoinsPerUtxoWord: "4310",
					KeyDeposit:       "200000",
				},
				Utxos: []types.Utxo{
					{
						Address: "addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5",
						Amounts: []types.Amount{
							{
								Unit:     "lovelace",
								Quantity: "5333004",
							},
						},
						TxHash: "72cfa181469b48402a50c6652d45c789897ae5025bb01f569a7bd01bffd12bc1",
						Index:  1,
					},
				},
				Slot:                    90_751_416,
				Fee:                     166265,
				TransactionValidityTime: 7200,
			},
			err: false,
		},
		{
			name:                       "ValidStakeInput",
			protocolParametersResponse: `{"min_fee_a":44,"min_fee_b":155381,"min_utxo":"4310", "coins_per_utxo_word":"4310","key_deposit":"200000"}`,
			addressUtxosResponse:       `[{"address":"addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5","tx_hash":"72cfa181469b48402a50c6652d45c789897ae5025bb01f569a7bd01bffd12bc1","tx_index":1,"output_index":1,"amount":[{"unit":"lovelace","quantity":"5333004"}],"block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","data_hash":null,"inline_datum":null,"reference_script_hash":null}]`,
			latestBlockResponse:        `{"time":1746434616,"height":3446505,"hash":"a13ea6bdb4f23caa803274011c0524e4d96eb7cacb4103998d508c257361cd1b","slot":90751416,"epoch":213,"epoch_slot":377016,"slot_leader":"pool1rccstu3l9ty3k0a5cd06fl3szsss9r34dcg5j38fqgq9kvng0tg","size":4,"tx_count":0,"output":null,"fees":null,"block_vrf":"vrf_vk1kkc5ar4jt2fkdcxp5sa0ekxsskfyjgp082xuqwcn7stvwr2dultsuwejcz","op_cert":"bb4f8eb8a07ca955b55a2c917df0475ccc36cf5487e1af0f97f562717d59ba82","op_cert_counter":"7","previous_block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","next_block":null,"confirmations":0}`,
			expectedInput: &tx_input.StakingInput{
				TxInput: tx_input.TxInput{
					ProtocolParams: types.ProtocolParameters{
						FeePerByte:       44,
						FixedFee:         155381,
						MinUtxoValue:     "4310",
						CoinsPerUtxoWord: "4310",
						KeyDeposit:       "200000",
					},
					Utxos: []types.Utxo{
						{
							Address: "addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5",
							Amounts: []types.Amount{
								{
									Unit:     "lovelace",
									Quantity: "5333004",
								},
							},
							TxHash: "72cfa181469b48402a50c6652d45c789897ae5025bb01f569a7bd01bffd12bc1",
							Index:  1,
						},
					},
					Slot: 90_751_416,
					// Fee should be greater than in standard tx - we need two signatured
					Fee:                     172337,
					TransactionValidityTime: 7200,
				},
				KeyDeposit: 200_000,
			},
			err: false,
		},
		{
			name:                       "InvalidKeyDeposit",
			protocolParametersResponse: `{"min_fee_a":44,"min_fee_b":155381,"min_utxo":"4310", "coins_per_utxo_word":"4310","key_deposit":200000}`,
			expectedInput:              &tx_input.StakingInput{},
			err:                        true,
		},
		{
			name:                       "ValidUnstakeInput",
			protocolParametersResponse: `{"min_fee_a":44,"min_fee_b":155381,"min_utxo":"4310", "coins_per_utxo_word":"4310","key_deposit":"200000"}`,
			addressUtxosResponse:       `[{"address":"addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5","tx_hash":"72cfa181469b48402a50c6652d45c789897ae5025bb01f569a7bd01bffd12bc1","tx_index":1,"output_index":1,"amount":[{"unit":"lovelace","quantity":"5333004"}],"block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","data_hash":null,"inline_datum":null,"reference_script_hash":null}]`,
			latestBlockResponse:        `{"time":1746434616,"height":3446505,"hash":"a13ea6bdb4f23caa803274011c0524e4d96eb7cacb4103998d508c257361cd1b","slot":90751416,"epoch":213,"epoch_slot":377016,"slot_leader":"pool1rccstu3l9ty3k0a5cd06fl3szsss9r34dcg5j38fqgq9kvng0tg","size":4,"tx_count":0,"output":null,"fees":null,"block_vrf":"vrf_vk1kkc5ar4jt2fkdcxp5sa0ekxsskfyjgp082xuqwcn7stvwr2dultsuwejcz","op_cert":"bb4f8eb8a07ca955b55a2c917df0475ccc36cf5487e1af0f97f562717d59ba82","op_cert_counter":"7","previous_block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","next_block":null,"confirmations":0}`,
			expectedInput: &tx_input.UnstakingInput{
				TxInput: tx_input.TxInput{
					ProtocolParams: types.ProtocolParameters{
						FeePerByte:       44,
						FixedFee:         155381,
						MinUtxoValue:     "4310",
						CoinsPerUtxoWord: "4310",
						KeyDeposit:       "200000",
					},
					Utxos: []types.Utxo{
						{
							Address: "addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5",
							Amounts: []types.Amount{
								{
									Unit:     "lovelace",
									Quantity: "5333004",
								},
							},
							TxHash: "72cfa181469b48402a50c6652d45c789897ae5025bb01f569a7bd01bffd12bc1",
							Index:  1,
						},
					},
					Slot: 90_751_416,
					// Fee should be greater than in standard tx, but lower than staking
					// We are PoolId part of the certificate
					Fee:                     171017,
					TransactionValidityTime: 7200,
				},
				KeyDeposit: 200_000,
			},
			err: false,
		},
		{
			name:                       "InvalidUnstakeKeyDeposit",
			protocolParametersResponse: `{"min_fee_a":44,"min_fee_b":155381,"min_utxo":"4310", "coins_per_utxo_word":"4310","key_deposit":200000}`,
			expectedInput:              &tx_input.UnstakingInput{},
			err:                        true,
		},
		{
			name:                       "ValidWithdrawInput",
			protocolParametersResponse: `{"min_fee_a":44,"min_fee_b":155381,"min_utxo":"4310", "coins_per_utxo_word":"4310","key_deposit":"200000"}`,
			addressUtxosResponse:       `[{"address":"addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5","tx_hash":"72cfa181469b48402a50c6652d45c789897ae5025bb01f569a7bd01bffd12bc1","tx_index":1,"output_index":1,"amount":[{"unit":"lovelace","quantity":"5333004"}],"block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","data_hash":null,"inline_datum":null,"reference_script_hash":null}]`,
			latestBlockResponse:        `{"time":1746434616,"height":3446505,"hash":"a13ea6bdb4f23caa803274011c0524e4d96eb7cacb4103998d508c257361cd1b","slot":90751416,"epoch":213,"epoch_slot":377016,"slot_leader":"pool1rccstu3l9ty3k0a5cd06fl3szsss9r34dcg5j38fqgq9kvng0tg","size":4,"tx_count":0,"output":null,"fees":null,"block_vrf":"vrf_vk1kkc5ar4jt2fkdcxp5sa0ekxsskfyjgp082xuqwcn7stvwr2dultsuwejcz","op_cert":"bb4f8eb8a07ca955b55a2c917df0475ccc36cf5487e1af0f97f562717d59ba82","op_cert_counter":"7","previous_block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","next_block":null,"confirmations":0}`,
			getAccountInfoResponse:     `{ "stake_address": "stake_test1upp33pdh0nppmxz8ma2def28nz8kju0yqrnmgfelcjf88fqd406dg", "active": true, "active_epoch": 246, "controlled_amount": "19996777537", "rewards_sum": "0", "withdrawals_sum": "0", "reserves_sum": "0", "treasury_sum": "0", "drep_id": null, "withdrawable_amount": "100000", "pool_id": "pool1m48d9wr228z4pn9rh2xw7d6d5a07sl2avej02c4vn54ujrnkz5e" }`,
			expectedInput: &tx_input.WithdrawInput{
				TxInput: tx_input.TxInput{
					ProtocolParams: types.ProtocolParameters{
						FeePerByte:       44,
						FixedFee:         155381,
						MinUtxoValue:     "4310",
						CoinsPerUtxoWord: "4310",
						KeyDeposit:       "200000",
					},
					Utxos: []types.Utxo{
						{
							Address: "addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5",
							Amounts: []types.Amount{
								{
									Unit:     "lovelace",
									Quantity: "5333004",
								},
							},
							TxHash: "72cfa181469b48402a50c6652d45c789897ae5025bb01f569a7bd01bffd12bc1",
							Index:  1,
						},
					},
					Slot:                    90_751_416,
					Fee:                     170753,
					TransactionValidityTime: 7200,
				},
				RewardsAddress: xc.Address("stake_test1upp33pdh0nppmxz8ma2def28nz8kju0yqrnmgfelcjf88fqd406dg"),
				RewardsAmount:  xc.NewAmountBlockchainFromUint64(100000),
			},
			err: false,
		},
	}

	cfg := NewTestConfig()
	for _, vector := range vectors {
		t.Run(vector.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				url := r.URL
				if vector.err {
					w.WriteHeader(http.StatusBadRequest)
				} else {
					w.WriteHeader(http.StatusOK)
				}
				var err error
				if strings.Contains(url.Path, "utxo") {
					_, err = w.Write([]byte(vector.addressUtxosResponse))
				} else if strings.Contains(url.Path, "block") {
					_, err = w.Write([]byte(vector.latestBlockResponse))
				} else if strings.Contains(url.Path, "account") {
					_, err = w.Write([]byte(vector.getAccountInfoResponse))
				} else {
					_, err = w.Write([]byte(vector.protocolParametersResponse))
				}
				require.NoError(t, err)
			}))
			defer server.Close()

			client, _ := client.NewClient(cfg)
			client.Url = server.URL

			var input xc.TxInput
			var err error
			if _, ok := vector.expectedInput.(*tx_input.TxInput); ok {
				args, _ := xcbuilder.NewTransferArgs(
					cfg.Base(),
					xc.Address("addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5"),
					xc.Address("addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5"),
					xc.NewAmountBlockchainFromUint64(1_000_000),
				)
				input, err = client.FetchTransferInput(context.Background(), args)
			} else if _, ok := vector.expectedInput.(*tx_input.StakingInput); ok {
				args, _ := xcbuilder.NewStakeArgs(
					xc.ADA,
					xc.Address("addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5"),

					xcbuilder.OptionValidator("dd4ed2b86a51c550cca3ba8cef374da75fe87d5d6664f562ac9d2bc9"),
					xcbuilder.OptionPublicKey(make([]byte, 32)),
				)
				input, err = client.FetchStakingInput(context.Background(), args)

			} else if _, ok := vector.expectedInput.(*tx_input.UnstakingInput); ok {
				args, _ := xcbuilder.NewStakeArgs(
					xc.ADA,
					xc.Address("addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5"),
					xcbuilder.OptionValidator("dd4ed2b86a51c550cca3ba8cef374da75fe87d5d6664f562ac9d2bc9"),
					xcbuilder.OptionPublicKey(make([]byte, 32)),
				)
				input, err = client.FetchUnstakingInput(context.Background(), args)
			} else if _, ok := vector.expectedInput.(*tx_input.WithdrawInput); ok {
				pk, _ := hex.DecodeString("f0bb6fd00a035b6b6ec18bbb2739265b80f319c0634333fe678928f40750cade")
				args, _ := xcbuilder.NewStakeArgs(
					xc.ADA,
					xc.Address("addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5"),
					xcbuilder.OptionPublicKey(pk),
					xcbuilder.OptionValidator("dd4ed2b86a51c550cca3ba8cef374da75fe87d5d6664f562ac9d2bc9"),
				)
				input, err = client.FetchWithdrawInput(context.Background(), args)
			}

			if vector.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, vector.expectedInput, input)

		})
	}
}

func TestSubmitTx(t *testing.T) {
	vectors := []struct {
		name                       string
		addressUtxosResponse       string
		latestBlockResponse        string
		protocolParametersResponse string
		submitSuccess              bool
		err                        bool
	}{
		{
			name:                       "FailedSubmit",
			addressUtxosResponse:       `[{"address":"addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5","tx_hash":"72cfa181469b48402a50c6652d45c789897ae5025bb01f569a7bd01bffd12bc1","tx_index":1,"output_index":1,"amount":[{"unit":"lovelace","quantity":"5333004"}],"block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","data_hash":null,"inline_datum":null,"reference_script_hash":null}]`,
			latestBlockResponse:        `{"time":1746434616,"height":3446505,"hash":"a13ea6bdb4f23caa803274011c0524e4d96eb7cacb4103998d508c257361cd1b","slot":90751416,"epoch":213,"epoch_slot":377016,"slot_leader":"pool1rccstu3l9ty3k0a5cd06fl3szsss9r34dcg5j38fqgq9kvng0tg","size":4,"tx_count":0,"output":null,"fees":null,"block_vrf":"vrf_vk1kkc5ar4jt2fkdcxp5sa0ekxsskfyjgp082xuqwcn7stvwr2dultsuwejcz","op_cert":"bb4f8eb8a07ca955b55a2c917df0475ccc36cf5487e1af0f97f562717d59ba82","op_cert_counter":"7","previous_block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","next_block":null,"confirmations":0}`,
			protocolParametersResponse: `{"min_fee_a":44,"min_fee_b":155381,"min_utxo":"4310", "coins_per_utxo_word":"4310"}`,
			submitSuccess:              false,
			err:                        true,
		},
		{
			name:                       "Success",
			addressUtxosResponse:       `[{"address":"addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5","tx_hash":"72cfa181469b48402a50c6652d45c789897ae5025bb01f569a7bd01bffd12bc1","tx_index":1,"output_index":1,"amount":[{"unit":"lovelace","quantity":"5333004"}],"block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","data_hash":null,"inline_datum":null,"reference_script_hash":null}]`,
			latestBlockResponse:        `{"time":1746434616,"height":3446505,"hash":"a13ea6bdb4f23caa803274011c0524e4d96eb7cacb4103998d508c257361cd1b","slot":90751416,"epoch":213,"epoch_slot":377016,"slot_leader":"pool1rccstu3l9ty3k0a5cd06fl3szsss9r34dcg5j38fqgq9kvng0tg","size":4,"tx_count":0,"output":null,"fees":null,"block_vrf":"vrf_vk1kkc5ar4jt2fkdcxp5sa0ekxsskfyjgp082xuqwcn7stvwr2dultsuwejcz","op_cert":"bb4f8eb8a07ca955b55a2c917df0475ccc36cf5487e1af0f97f562717d59ba82","op_cert_counter":"7","previous_block":"babb05fe6f3128a398dfce79ad1f836a0031bd6d84969755ce7a043b0c604cec","next_block":null,"confirmations":0}`,
			protocolParametersResponse: `{"min_fee_a":44,"min_fee_b":155381,"min_utxo":"4310", "coins_per_utxo_word":"4310"}`,
			submitSuccess:              true,
			err:                        false,
		},
	}

	cfg := NewTestConfig()
	for _, vector := range vectors {
		t.Run(vector.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				url := r.URL
				var err error
				if strings.Contains(url.Path, "utxo") {
					_, err = w.Write([]byte(vector.addressUtxosResponse))
				} else if strings.Contains(url.Path, "block") {
					_, err = w.Write([]byte(vector.latestBlockResponse))
				} else if strings.Contains(url.Path, "submit") {
					if vector.submitSuccess {
						w.WriteHeader(http.StatusOK)
					} else {
						w.WriteHeader(http.StatusBadRequest)
					}
				} else {
					_, err = w.Write([]byte(vector.protocolParametersResponse))
				}
				require.NoError(t, err)
			}))
			defer server.Close()

			client, _ := client.NewClient(cfg)
			client.Url = server.URL

			args, err := xcbuilder.NewTransferArgs(
				cfg.Base(),
				xc.Address("addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5"),
				xc.Address("addr_test1qrfp5xelv2mu7k8zyvwm0c8t5xm55wanwhtd4fgjgtf3ck0rplhn7x9jyhwqg70fwv0ujpmyumqk5td9e9hnsejtlxnq3yqf25"),
				xc.NewAmountBlockchainFromUint64(1_000_000),
			)
			require.NoError(t, err)
			input, err := client.FetchTransferInput(context.Background(), args)
			require.NoError(t, err)
			require.NotNil(t, input)

			tx, err := tx.NewTransfer(args, input)
			require.NoError(t, err)

			err = client.SubmitTx(context.Background(), tx)
			if vector.err {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestFetchTxInfo(t *testing.T) {
	vectors := []struct {
		name                   string
		getLatestBlockResponse string
		getTxInfoResponse      string
		getBlockInfo           string
		getTxUtxos             string
		expectedInfo           xclient.TxInfo
	}{
		{
			name:                   "ValidResponse",
			getLatestBlockResponse: `{"time":1746445795,"height":3446919,"hash":"9d5440776dd6432ae89db73241ff35b2d603ad48e040f5ea9897df3f3251cb6d","slot":90762595,"epoch":213,"epoch_slot":388195,"slot_leader":"pool1z05xqzuxnpl8kg8u2wwg8ftng0fwtdluv3h20ruryfqc5gc3efl","size":4,"tx_count":0,"output":null,"fees":null,"block_vrf":"vrf_vk1n9jgutq5vr79dhftlzwvmppa8myt4x94u3znhylud7vc0wts0g4q9v68z8","op_cert":"de399288bef696641b7bbb25ffd9ee476e4facb2aa3f2f66e4cb01cd2964537c","op_cert_counter":"12","previous_block":"a407a024a531936330b069d764748ef48521dd495242636f56df322815b510fd","next_block":null,"confirmations":0}`,
			getTxInfoResponse:      `{"hash":"3b28dc6d6cc32280a2738799a6b5defc96b66999a17481a7cdaa27e8c00bd610","block":"fc83ca908ded9ae5043276fcc54ea7438d0d2b02a429e80ce48aca57c2d08623","block_height":3436192,"block_time":1746168211,"slot":90485011,"index":0,"output_amount":[{"unit":"lovelace","quantity":"19758792833"},{"unit":"2682a9b99553406c39f693bb450d6001954e3504c96238c6b96ad79a476c6f62616c20456e7465727461696e6d656e7420546f6b656e202847455429","quantity":"210000"},{"unit":"2682a9b99553406c39f693bb450d6001954e3504c96238c6b96ad79a476c6f62616c20456e7465727461696e6d656e7420546f6b656e202847455429","quantity":"194249999"},{"unit":"553cbd1862bfb5ff69b6ebdcbb6165beb47171a04b841c79c72e75a94745542d415554482d544b4e","quantity":"1"}],"fees":"1402193","deposit":"0","size":13268,"invalid_before":null,"invalid_hereafter":null,"utxo_count":4,"withdrawal_count":0,"mir_cert_count":0,"delegation_count":0,"stake_cert_count":0,"pool_update_count":0,"pool_retire_count":0,"asset_mint_or_burn_count":0,"redeemer_count":1,"valid_contract":true}`,
			getBlockInfo:           `{"time":1746168211,"height":3436192,"hash":"fc83ca908ded9ae5043276fcc54ea7438d0d2b02a429e80ce48aca57c2d08623","slot":90485011,"epoch":213,"epoch_slot":110611,"slot_leader":"pool1ytvs2jlrtftapsgdhtlm8ch0xa2d3e3lsnf59jz68mtk5pml699","size":13272,"tx_count":1,"output":"19758792833","fees":"1402193","block_vrf":"vrf_vk1cham4jns9uc5rg0yk7fufpyc2pcz67tnhyh96g48l2u00skkmrlqnp8xmp","op_cert":"fa27bd7afb410ac7d9e16d6da1a4b487305424289336d0a0035f52bb3b3830c8","op_cert_counter":"2","previous_block":"8b58c40edccfbe2b852ed75e4c4f680da383fcc3298a177db84d80ba78a5b94b","next_block":"962e871831aa0fcdbb457bcf147a553df4ab87d32a7fcfd09901cd2a331d750f","confirmations":10727}`,
			getTxUtxos:             `{"hash":"3b28dc6d6cc32280a2738799a6b5defc96b66999a17481a7cdaa27e8c00bd610","inputs":[{"address":"addr_test1wr54sl5p9yuvaknwv8kjyg7k7n6h02rccyh7s7ntuqs49rsfy5283","amount":[{"unit":"lovelace","quantity":"1796623"},{"unit":"2682a9b99553406c39f693bb450d6001954e3504c96238c6b96ad79a476c6f62616c20456e7465727461696e6d656e7420546f6b656e202847455429","quantity":"10000"},{"unit":"553cbd1862bfb5ff69b6ebdcbb6165beb47171a04b841c79c72e75a94745542d415554482d544b4e","quantity":"1"}],"tx_hash":"bcda48666026306d1b958256cb94ad4cef0fe14d1dc44165b25947642e1eae1e","output_index":0,"data_hash":"2d9bfbb2e1b6e1af2f55f268a3ad8281f218661fd38c66abf91ed8d9f69a3cf7","inline_datum":"d8799f43474554192710192710581c2682a9b99553406c39f693bb450d6001954e3504c96238c6b96ad79a5820476c6f62616c20456e7465727461696e6d656e7420546f6b656e202847455429ff","reference_script_hash":null,"collateral":false,"reference":false},{"address":"addr_test1qzyf6t5qq037n0srm4r84w3hchgvmgu45ks9pf5f27qhnykxp572eea783cccv2fmpqs5s6va4n7pusy097meenje88s6p7x42","amount":[{"unit":"lovelace","quantity":"19758398403"},{"unit":"2682a9b99553406c39f693bb450d6001954e3504c96238c6b96ad79a476c6f62616c20456e7465727461696e6d656e7420546f6b656e202847455429","quantity":"194449999"}],"tx_hash":"bcda48666026306d1b958256cb94ad4cef0fe14d1dc44165b25947642e1eae1e","output_index":1,"data_hash":null,"inline_datum":null,"reference_script_hash":null,"collateral":false,"reference":false},{"address":"addr_test1qqdqfz660junmjs96qxyh760e9h6zme5jrvectx4tznhk8q72rs5hptlwsvhwphrfrkuyftnxwv6ld2r8yag3gmaz82sx05amf","amount":[{"unit":"lovelace","quantity":"489053618"}],"tx_hash":"6cda8edb61295ddd017ec83f330f049e0704a622b78484a3361865e12837fbd3","output_index":3,"data_hash":null,"inline_datum":null,"reference_script_hash":null,"collateral":true,"reference":false}],"outputs":[{"address":"addr_test1wr54sl5p9yuvaknwv8kjyg7k7n6h02rccyh7s7ntuqs49rsfy5283","amount":[{"unit":"lovelace","quantity":"1796623"},{"unit":"2682a9b99553406c39f693bb450d6001954e3504c96238c6b96ad79a476c6f62616c20456e7465727461696e6d656e7420546f6b656e202847455429","quantity":"210000"},{"unit":"553cbd1862bfb5ff69b6ebdcbb6165beb47171a04b841c79c72e75a94745542d415554482d544b4e","quantity":"1"}],"output_index":0,"data_hash":"db96855ad4f7d869a68d8b236dd2c80ef6c810fa34ee150150d3274aef4250ce","inline_datum":"d8799f434745541a000334501a00033450581c2682a9b99553406c39f693bb450d6001954e3504c96238c6b96ad79a5820476c6f62616c20456e7465727461696e6d656e7420546f6b656e202847455429ff","collateral":false,"reference_script_hash":null,"consumed_by_tx":"b32a89380c217f3aede4b95d5acc7cd9d5e704d4f3fb924b9ffa6b3558494978"},{"address":"addr_test1qzyf6t5qq037n0srm4r84w3hchgvmgu45ks9pf5f27qhnykxp572eea783cccv2fmpqs5s6va4n7pusy097meenje88s6p7x42","amount":[{"unit":"lovelace","quantity":"19756996210"},{"unit":"2682a9b99553406c39f693bb450d6001954e3504c96238c6b96ad79a476c6f62616c20456e7465727461696e6d656e7420546f6b656e202847455429","quantity":"194249999"}],"output_index":1,"data_hash":null,"inline_datum":null,"collateral":false,"reference_script_hash":null,"consumed_by_tx":"b32a89380c217f3aede4b95d5acc7cd9d5e704d4f3fb924b9ffa6b3558494978"}]}`,
			expectedInfo: xclient.TxInfo{
				Name:   "chains/ADA/transactions/3b28dc6d6cc32280a2738799a6b5defc96b66999a17481a7cdaa27e8c00bd610",
				Hash:   "3b28dc6d6cc32280a2738799a6b5defc96b66999a17481a7cdaa27e8c00bd610",
				XChain: "ADA",
				State:  "succeeded",
				Final:  true,
				Block: &xclient.Block{
					Chain:  "ADA",
					Height: xc.NewAmountBlockchainFromUint64(3_436_192),
					Hash:   "fc83ca908ded9ae5043276fcc54ea7438d0d2b02a429e80ce48aca57c2d08623",
					Time:   MustParseTime("2025-05-02T08:43:31+02:00"),
				},
				Movements: []*xclient.Movement{
					NewMovement(
						"ADA",
						"2682a9b99553406c39f693bb450d6001954e3504c96238c6b96ad79a476c6f62616c20456e7465727461696e6d656e7420546f6b656e202847455429",
						[]*xclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(10_000),
								XAddress:  "chains/ADA/addresses/addr_test1wr54sl5p9yuvaknwv8kjyg7k7n6h02rccyh7s7ntuqs49rsfy5283",
								AddressId: "addr_test1wr54sl5p9yuvaknwv8kjyg7k7n6h02rccyh7s7ntuqs49rsfy5283",
							},
							{
								Balance:   xc.NewAmountBlockchainFromUint64(194_449_999),
								XAddress:  "chains/ADA/addresses/addr_test1qzyf6t5qq037n0srm4r84w3hchgvmgu45ks9pf5f27qhnykxp572eea783cccv2fmpqs5s6va4n7pusy097meenje88s6p7x42",
								AddressId: "addr_test1qzyf6t5qq037n0srm4r84w3hchgvmgu45ks9pf5f27qhnykxp572eea783cccv2fmpqs5s6va4n7pusy097meenje88s6p7x42",
							},
						},
						[]*xclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(210_000),
								XAddress:  "chains/ADA/addresses/addr_test1wr54sl5p9yuvaknwv8kjyg7k7n6h02rccyh7s7ntuqs49rsfy5283",
								AddressId: "addr_test1wr54sl5p9yuvaknwv8kjyg7k7n6h02rccyh7s7ntuqs49rsfy5283",
							},
							{
								Balance:   xc.NewAmountBlockchainFromUint64(194_249_999),
								XAddress:  "chains/ADA/addresses/addr_test1qzyf6t5qq037n0srm4r84w3hchgvmgu45ks9pf5f27qhnykxp572eea783cccv2fmpqs5s6va4n7pusy097meenje88s6p7x42",
								AddressId: "addr_test1qzyf6t5qq037n0srm4r84w3hchgvmgu45ks9pf5f27qhnykxp572eea783cccv2fmpqs5s6va4n7pusy097meenje88s6p7x42",
							},
						},
						nil,
					),
					NewMovement(
						"ADA",
						"553cbd1862bfb5ff69b6ebdcbb6165beb47171a04b841c79c72e75a94745542d415554482d544b4e",
						[]*xclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(1),
								XAddress:  "chains/ADA/addresses/addr_test1wr54sl5p9yuvaknwv8kjyg7k7n6h02rccyh7s7ntuqs49rsfy5283",
								AddressId: "addr_test1wr54sl5p9yuvaknwv8kjyg7k7n6h02rccyh7s7ntuqs49rsfy5283",
							},
						},
						[]*xclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(1),
								XAddress:  "chains/ADA/addresses/addr_test1wr54sl5p9yuvaknwv8kjyg7k7n6h02rccyh7s7ntuqs49rsfy5283",
								AddressId: "addr_test1wr54sl5p9yuvaknwv8kjyg7k7n6h02rccyh7s7ntuqs49rsfy5283",
							},
						},
						nil,
					),
					NewMovement(
						"ADA",
						"ADA",
						[]*xclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(1_796_623),
								XAddress:  "chains/ADA/addresses/addr_test1wr54sl5p9yuvaknwv8kjyg7k7n6h02rccyh7s7ntuqs49rsfy5283",
								AddressId: "addr_test1wr54sl5p9yuvaknwv8kjyg7k7n6h02rccyh7s7ntuqs49rsfy5283",
							},
							{
								Balance:   xc.NewAmountBlockchainFromUint64(19_758_398_403),
								XAddress:  "chains/ADA/addresses/addr_test1qzyf6t5qq037n0srm4r84w3hchgvmgu45ks9pf5f27qhnykxp572eea783cccv2fmpqs5s6va4n7pusy097meenje88s6p7x42",
								AddressId: "addr_test1qzyf6t5qq037n0srm4r84w3hchgvmgu45ks9pf5f27qhnykxp572eea783cccv2fmpqs5s6va4n7pusy097meenje88s6p7x42",
							},
							{
								Balance:   xc.NewAmountBlockchainFromUint64(489_053_618),
								XAddress:  "chains/ADA/addresses/addr_test1qqdqfz660junmjs96qxyh760e9h6zme5jrvectx4tznhk8q72rs5hptlwsvhwphrfrkuyftnxwv6ld2r8yag3gmaz82sx05amf",
								AddressId: "addr_test1qqdqfz660junmjs96qxyh760e9h6zme5jrvectx4tznhk8q72rs5hptlwsvhwphrfrkuyftnxwv6ld2r8yag3gmaz82sx05amf",
							},
						},
						[]*xclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(1_796_623),
								XAddress:  "chains/ADA/addresses/addr_test1wr54sl5p9yuvaknwv8kjyg7k7n6h02rccyh7s7ntuqs49rsfy5283",
								AddressId: "addr_test1wr54sl5p9yuvaknwv8kjyg7k7n6h02rccyh7s7ntuqs49rsfy5283",
							},
							{
								Balance:   xc.NewAmountBlockchainFromUint64(19_756_996_210),
								XAddress:  "chains/ADA/addresses/addr_test1qzyf6t5qq037n0srm4r84w3hchgvmgu45ks9pf5f27qhnykxp572eea783cccv2fmpqs5s6va4n7pusy097meenje88s6p7x42",
								AddressId: "addr_test1qzyf6t5qq037n0srm4r84w3hchgvmgu45ks9pf5f27qhnykxp572eea783cccv2fmpqs5s6va4n7pusy097meenje88s6p7x42",
							},
						},
						nil,
					),
				},
				Fees: []*xclient.Balance{
					{
						Asset:    "chains/ADA/assets/ADA",
						Contract: "ADA",
						Balance:  xc.NewAmountBlockchainFromUint64(1_402_193),
					},
				},
				Stakes:        nil,
				Unstakes:      nil,
				Confirmations: 10727,
				Error:         nil,
			},
		},
	}

	for _, vector := range vectors {
		t.Run(vector.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				url := r.URL
				var err error
				if strings.Contains(url.Path, "blocks/latest") {
					_, err = w.Write([]byte(vector.getLatestBlockResponse))
				} else if strings.Contains(url.Path, "utxos") {
					_, err = w.Write([]byte(vector.getTxUtxos))
				} else if strings.Contains(url.Path, "txs") {
					_, err = w.Write([]byte(vector.getTxInfoResponse))
				} else if strings.Contains(url.Path, "blocks") {
					_, err = w.Write([]byte(vector.getBlockInfo))
				}
				require.NoError(t, err)
			}))
			defer server.Close()

			cfg := NewTestConfig()
			client, _ := client.NewClient(cfg)
			client.Url = server.URL

			args := txinfo.NewArgs(xc.TxHash("3b28dc6d6cc32280a2738799a6b5defc96b66999a17481a7cdaa27e8c00bd610"))
			txInfo, err := client.FetchTxInfo(context.Background(), args)
			require.NoError(t, err)
			require.Equal(t, vector.expectedInfo, txInfo)
		})
	}

}

func MustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic("failed to parse time")
	}

	unixTime := t.Unix()
	return time.Unix(unixTime, 0)
}

func NewMovement(
	chain string,
	contract string,
	from []*xclient.BalanceChange,
	to []*xclient.BalanceChange,
	event *xclient.Event,
) *xclient.Movement {
	movement := xclient.NewMovement(xc.NativeAsset(chain), xc.ContractAddress(contract))
	movement.From = from
	movement.To = to
	movement.Event = event
	return movement
}
