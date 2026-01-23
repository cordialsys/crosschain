package tron_test

import (
	"crypto/rand"
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/tron"
	"github.com/cordialsys/crosschain/chain/tron/core"
	httpclient "github.com/cordialsys/crosschain/chain/tron/http_client"
	"github.com/cordialsys/crosschain/chain/tron/txinput"
	"github.com/cordialsys/crosschain/testutil"
	"github.com/stretchr/testify/require"
)

func randomSig() []byte {
	sig := make([]byte, 65)
	_, _ = rand.Read(sig)
	return sig
}

func TestNewTxBuilder(t *testing.T) {
	type testcase struct {
		name                 string
		input                xc.TxInput
		args                 buildertest.TransferArgs
		stake_args           buildertest.StakeArgs
		expectedSigHex       []string
		expectedTransactions []core.Transaction_Contract_ContractType
	}
	chainCfg := xc.NewChainConfig(xc.TRX).WithDecimals(6).Base()

	testcases := []testcase{
		{
			name: "native transfer",
			input: &txinput.TxInput{
				TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
				RefBlockBytes:   testutil.FromHex("5273"),
				RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
				Expiration:      200,
				Timestamp:       100,
				MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
			},
			args: buildertest.MustNewTransferArgs(
				chainCfg,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
				xc.Address("TUz4nTU75z5oK4pYaVipkSDQ3Bi2DXdQT8"),
				xc.NewAmountBlockchainFromUint64(10000),
			),
			expectedSigHex:       []string{"bffb93894087cb83be9a9546afb83da420cb67fceefb99a28316532ffa4c9ede"},
			expectedTransactions: []core.Transaction_Contract_ContractType{core.Transaction_Contract_TransferContract},
		},
		{
			name: "token transfer",
			input: &txinput.TxInput{
				TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
				RefBlockBytes:   testutil.FromHex("5273"),
				RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
				Expiration:      200,
				Timestamp:       100,
				MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
			},
			args: buildertest.MustNewTransferArgs(
				chainCfg,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
				xc.Address("TUz4nTU75z5oK4pYaVipkSDQ3Bi2DXdQT8"),
				xc.NewAmountBlockchainFromUint64(10000),
				buildertest.OptionContractAddress("TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs"),
				buildertest.OptionContractDecimals(6),
			),
			expectedSigHex:       []string{"a6d38223922a3210653eb97b71bd98f47eebecbbc5fc5f3d60dbadc69c68ee68"},
			expectedTransactions: []core.Transaction_Contract_ContractType{core.Transaction_Contract_TriggerSmartContract},
		},
		{
			name: "full stake",
			input: &txinput.StakeInput{
				TxInput: txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				FreezeInput: &txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				Votes: []*httpclient.Vote{},
				// No freezed balance means that we will have to freeze + vote
				FreezedBalance: 0,
				Decimals:       6,
			},
			stake_args: buildertest.MustNewStakingArgs(
				chainCfg.Chain,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
				builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(1_000_000)),
				builder.OptionValidator("TUz4nTU75z5oK4pYaVipkSDQ3Bi2DXdQT8"),
			),
			expectedSigHex: []string{
				"9079390c296b29a667fbf333494f34f993c1e5ee52bdbe9d83442b8414b7655c",
				"6d2116a25cf575b9e0e4a9f7337c6ef3d72024cdba43ec27b2e0356d5b281175",
			},
			expectedTransactions: []core.Transaction_Contract_ContractType{
				core.Transaction_Contract_FreezeBalanceV2Contract,
				core.Transaction_Contract_VoteWitnessContract,
			},
		},
		{
			name: "vote only stake",
			input: &txinput.StakeInput{
				TxInput: txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				FreezeInput: &txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				Votes: []*httpclient.Vote{},
				// Freezed balance means that we will be able to vote, without explicit freeze call
				FreezedBalance: 2_000_000,
				Decimals:       6,
			},
			stake_args: buildertest.MustNewStakingArgs(
				chainCfg.Chain,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
				builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(1_000_000)),
				builder.OptionValidator("TUz4nTU75z5oK4pYaVipkSDQ3Bi2DXdQT8"),
			),
			expectedSigHex: []string{
				"6d2116a25cf575b9e0e4a9f7337c6ef3d72024cdba43ec27b2e0356d5b281175",
			},
			expectedTransactions: []core.Transaction_Contract_ContractType{
				core.Transaction_Contract_VoteWitnessContract,
			},
		},
		{
			name: "vote only stake, no freeze input",
			input: &txinput.StakeInput{
				TxInput: txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				FreezeInput: nil,
				Votes:       []*httpclient.Vote{},
				// Freezed balance means that we will be able to vote, without explicit freeze call
				FreezedBalance: 2_000_000,
				Decimals:       6,
			},
			stake_args: buildertest.MustNewStakingArgs(
				chainCfg.Chain,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
				builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(1_000_000)),
				builder.OptionValidator("TUz4nTU75z5oK4pYaVipkSDQ3Bi2DXdQT8"),
			),
			expectedSigHex: []string{
				"6d2116a25cf575b9e0e4a9f7337c6ef3d72024cdba43ec27b2e0356d5b281175",
			},
			expectedTransactions: []core.Transaction_Contract_ContractType{
				core.Transaction_Contract_VoteWitnessContract,
			},
		},
		{
			name: "full unstake",
			input: &txinput.UnstakeInput{
				TxInput: txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				VoteInput: &txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				Votes: []*httpclient.Vote{
					{
						VoteAddress: "TJmka325yjJKeFpQDwKSQAoNwEyNGhsaEV",
						VoteCount:   2,
					},
				},
				// No freezed balance means that we will have to vote, and only then unfreeze
				FreezedBalance: 0,
				Decimals:       6,
			},
			stake_args: buildertest.MustNewStakingArgs(
				chainCfg.Chain,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
				builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(1_000_000)),
				builder.OptionValidator("TJmka325yjJKeFpQDwKSQAoNwEyNGhsaEV"),
			),
			expectedSigHex: []string{
				"e7058780953a68d73308b609bde6dd661cb06367e75a07f836c6524ed019d130",
				"3b382c12231fcd509583eff02be2ca6c5843ea4f06eaad74452a5e4737eaea26",
			},
			expectedTransactions: []core.Transaction_Contract_ContractType{
				core.Transaction_Contract_VoteWitnessContract,
				core.Transaction_Contract_UnfreezeBalanceV2Contract,
			},
		},
		{
			name: "unfreeze only unstake",
			input: &txinput.UnstakeInput{
				TxInput: txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				VoteInput: &txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				Votes: []*httpclient.Vote{
					{
						VoteAddress: "TJmka325yjJKeFpQDwKSQAoNwEyNGhsaEV",
						VoteCount:   2,
					},
				},
				// Freezed balance means that we can unfreeze if no validator is specified
				// Please note that freezed balance should be greater than used votes
				// In this case:
				// 2 votes = 2TRX
				// 3 TRX freezed = 3 total votes
				// 1 TRX is left to unfreeze
				FreezedBalance: 3_000_000,
				Decimals:       6,
			},
			stake_args: buildertest.MustNewStakingArgs(
				chainCfg.Chain,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
				builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(1_000_000)),
				// no validator
			),
			expectedSigHex: []string{
				"3b382c12231fcd509583eff02be2ca6c5843ea4f06eaad74452a5e4737eaea26",
			},
			expectedTransactions: []core.Transaction_Contract_ContractType{
				core.Transaction_Contract_UnfreezeBalanceV2Contract,
			},
		},
		{
			name: "unfreeze only unstake, no vote input",
			input: &txinput.UnstakeInput{
				TxInput: txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				VoteInput: nil,
				Votes: []*httpclient.Vote{
					{
						VoteAddress: "TJmka325yjJKeFpQDwKSQAoNwEyNGhsaEV",
						VoteCount:   2,
					},
				},
				// Freezed balance means that we can unfreeze if no validator is specified
				// Please note that freezed balance should be greater than used votes
				// In this case:
				// 2 votes = 2TRX
				// 3 TRX freezed = 3 total votes
				// 1 TRX is left to unfreeze
				FreezedBalance: 3_000_000,
				Decimals:       6,
			},
			stake_args: buildertest.MustNewStakingArgs(
				chainCfg.Chain,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
				builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(1_000_000)),
				// no validator
			),
			expectedSigHex: []string{
				"3b382c12231fcd509583eff02be2ca6c5843ea4f06eaad74452a5e4737eaea26",
			},
			expectedTransactions: []core.Transaction_Contract_ContractType{
				core.Transaction_Contract_UnfreezeBalanceV2Contract,
			},
		},
		{
			name: "explicit validator unstake",
			input: &txinput.UnstakeInput{
				TxInput: txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				VoteInput: &txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				Votes: []*httpclient.Vote{
					{
						VoteAddress: "TJmka325yjJKeFpQDwKSQAoNwEyNGhsaEV",
						VoteCount:   2,
					},
				},
				// We have 1 TRX left to unfreeze (2 votes, 3 freezed balance)
				// However validator was explicitly passed, meaning that we have to unvote first
				FreezedBalance: 3_000_000,
				Decimals:       6,
			},
			stake_args: buildertest.MustNewStakingArgs(
				chainCfg.Chain,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
				builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(1_000_000)),
				builder.OptionValidator("TJmka325yjJKeFpQDwKSQAoNwEyNGhsaEV"),
				// no validator
			),
			expectedSigHex: []string{
				"e7058780953a68d73308b609bde6dd661cb06367e75a07f836c6524ed019d130",
				"3b382c12231fcd509583eff02be2ca6c5843ea4f06eaad74452a5e4737eaea26",
			},
			expectedTransactions: []core.Transaction_Contract_ContractType{
				core.Transaction_Contract_VoteWitnessContract,
				core.Transaction_Contract_UnfreezeBalanceV2Contract,
			},
		},
		{
			name: "no validator unstake",
			input: &txinput.UnstakeInput{
				TxInput: txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				VoteInput: &txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				Votes: []*httpclient.Vote{
					{
						VoteAddress: "TJmka325yjJKeFpQDwKSQAoNwEyNGhsaEV",
						VoteCount:   2,
					},
				},
				// We have 1 TRX left to unfreeze (2 votes, 3 freezed balance)
				// However user asked to unstake 2.0 TRX. We have to vote first to unfreeze
				FreezedBalance: 3_000_000,
				Decimals:       6,
			},
			stake_args: buildertest.MustNewStakingArgs(
				chainCfg.Chain,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
				builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(1_000_000)),
				builder.OptionValidator("TJmka325yjJKeFpQDwKSQAoNwEyNGhsaEV"),
				// no validator
			),
			expectedSigHex: []string{
				"e7058780953a68d73308b609bde6dd661cb06367e75a07f836c6524ed019d130",
				"3b382c12231fcd509583eff02be2ca6c5843ea4f06eaad74452a5e4737eaea26",
			},
			expectedTransactions: []core.Transaction_Contract_ContractType{
				core.Transaction_Contract_VoteWitnessContract,
				core.Transaction_Contract_UnfreezeBalanceV2Contract,
			},
		},
		{
			name: "full withdraw",
			input: &txinput.WithdrawInput{
				TxInput: &txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
				WithdrawRewardsInput: &txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
			},
			stake_args: buildertest.MustNewStakingArgs(
				chainCfg.Chain,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
			),
			expectedSigHex: []string{
				"7aec0238760d5bde13f15e9673d8a144e4e1c577746546f2e2352378d670a25a",
				"69e6c84517b22a6a9b520a46f4d3846675ae507ac24904b201e905fef59addba",
			},
			expectedTransactions: []core.Transaction_Contract_ContractType{
				core.Transaction_Contract_WithdrawExpireUnfreezeContract,
				core.Transaction_Contract_WithdrawBalanceContract,
			},
		},
		{
			name: "unfreeze only withdraw",
			input: &txinput.WithdrawInput{
				TxInput: &txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
			},
			stake_args: buildertest.MustNewStakingArgs(
				chainCfg.Chain,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
			),
			expectedSigHex: []string{
				"7aec0238760d5bde13f15e9673d8a144e4e1c577746546f2e2352378d670a25a",
			},
			expectedTransactions: []core.Transaction_Contract_ContractType{
				core.Transaction_Contract_WithdrawExpireUnfreezeContract,
			},
		},
		{
			name: "rewards only withdraw",
			input: &txinput.WithdrawInput{
				TxInput: nil,
				WithdrawRewardsInput: &txinput.TxInput{
					TxInputEnvelope: txinput.NewTxInput().TxInputEnvelope,
					RefBlockBytes:   testutil.FromHex("5273"),
					RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
					Expiration:      200,
					Timestamp:       100,
					MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
				},
			},
			stake_args: buildertest.MustNewStakingArgs(
				chainCfg.Chain,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
			),
			expectedSigHex: []string{
				"69e6c84517b22a6a9b520a46f4d3846675ae507ac24904b201e905fef59addba",
			},
			expectedTransactions: []core.Transaction_Contract_ContractType{
				core.Transaction_Contract_WithdrawBalanceContract,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			txBuilder, err := tron.NewTxBuilder(chainCfg)
			require.NoError(t, err)
			require.NotNil(t, txBuilder)

			var txI xc.Tx
			switch i := tc.input.(type) {
			case *txinput.StakeInput:
				txI, err = txBuilder.Stake(tc.stake_args, i)
			case *txinput.UnstakeInput:
				txI, err = txBuilder.Unstake(tc.stake_args, i)
			case *txinput.WithdrawInput:
				txI, err = txBuilder.Withdraw(tc.stake_args, i)
			case *txinput.TxInput:
				txI, err = txBuilder.Transfer(tc.args, tc.input)
			}
			require.NoError(t, err)
			require.NotNil(t, txI)

			hashes, err := txI.Sighashes()
			require.NoError(t, err)
			require.Equal(t, len(tc.expectedTransactions), len(hashes))

			hexHashes := make([]string, 0)
			for _, h := range hashes {
				hexHashes = append(hexHashes, hex.EncodeToString(h.Payload))
			}
			require.Equal(t, tc.expectedSigHex, hexHashes)

			sigs := make([]*xc.SignatureResponse, 0)
			for _ = range len(hashes) {
				sigs = append(sigs, &xc.SignatureResponse{
					Signature: randomSig(),
				})
			}

			err = txI.SetSignatures(sigs...)
			require.NoError(t, err)

			bz, err := txI.Serialize()
			require.NoError(t, err)
			require.True(t, len(bz) > 0)

			_, ok := txI.(xc.TxWithMetadata)
			require.True(t, ok, "tron transactions should implement TxWithMetadata")

			tx, ok := txI.(*tron.Tx)
			require.True(t, ok)

			require.Equal(t, len(tc.expectedTransactions), len(tx.TronTxs))
			for i, etx := range tc.expectedTransactions {
				require.Equal(t, etx, tx.TronTxs[i].RawData.Contract[0].Type)
			}
		})
	}
}
