package builder_test

import (
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/solana/builder"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"
)

func TestNewStakingTransfer(t *testing.T) {

	txBuilder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())

	from := xc.Address("83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH") // fails on parsing from
	validator := "J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp"

	amount := xc.NewAmountBlockchainFromUint64(100000000)
	args := buildertest.MustNewStakingArgs(xc.SOL, from, amount, xcbuilder.OptionValidator(validator))

	stakeKey, _ := solana.NewRandomPrivateKey()

	input := &tx_input.StakingInput{
		TxInput: tx_input.TxInput{
			RecentBlockHash:   solana.MustHashFromBase58("DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK"),
			PrioritizationFee: xc.NewAmountBlockchainFromUint64(100000),
		},
		ValidatorVoteAccount: solana.MustPublicKeyFromBase58(validator),
		StakingKey:           stakeKey,
	}

	tx, err := txBuilder.Stake(args, input)
	require.NoError(t, err)
	require.NotNil(t, tx)

	createAccounts := tx.(*Tx).GetCreateAccounts()
	stakes := tx.(*Tx).GetDelegateStake()
	require.Len(t, createAccounts, 1)
	require.Len(t, stakes, 1)

	// new account initialized with the amount
	require.Equal(t, amount.Uint64(), createAccounts[0].Instruction.Lamports)
	require.Equal(t, stakeKey.PublicKey(), createAccounts[0].Instruction.NewAccount)

	// delegated to validator
	require.Equal(t, input.ValidatorVoteAccount, stakes[0].Instruction.GetVoteAccount().PublicKey)
	require.Equal(t, stakeKey.PublicKey(), stakes[0].Instruction.GetStakeAccount().PublicKey)
}

func TestNewUnstakeTransfer(t *testing.T) {

	txBuilder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())

	from := xc.Address("83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH") // fails on parsing from
	validator := "50_000_000_000"

	amount := xc.NewAmountBlockchainFromUint64(85_000_000_000)
	args := buildertest.MustNewStakingArgs(xc.SOL, from, amount, xcbuilder.OptionValidator(validator))

	stakeKey, _ := solana.NewRandomPrivateKey()

	input := &tx_input.UnstakingInput{
		TxInput: tx_input.TxInput{
			RecentBlockHash:   solana.MustHashFromBase58("DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK"),
			PrioritizationFee: xc.NewAmountBlockchainFromUint64(100000),
		},
		StakingKey: stakeKey,
		// 90 SOL total staked
		EligibleStakes: []*tx_input.ExistingStake{
			{
				ActivationEpoch:   xc.NewAmountBlockchainFromUint64(650),
				DeactivationEpoch: xc.NewAmountBlockchainFromUint64(18446744073709551615),
				AmountActive:      xc.NewAmountBlockchainFromUint64(50_000_000_000),
				AmountInactive:    xc.NewAmountBlockchainFromUint64(2282880),
				StakeAccount:      solana.MustPublicKeyFromBase58("CCTFhyxoUHGmdQvuUxFquyYMK4H5hdqwCCN7XAXtK9HC"),
			},
			{
				ActivationEpoch:   xc.NewAmountBlockchainFromUint64(650),
				DeactivationEpoch: xc.NewAmountBlockchainFromUint64(18446744073709551615),
				AmountActive:      xc.NewAmountBlockchainFromUint64(30_000_000_000),
				AmountInactive:    xc.NewAmountBlockchainFromUint64(2282880),
				StakeAccount:      solana.MustPublicKeyFromBase58("8zrSGLMdE6dK57Q7a8N8TDohmyft1MrsLYdRqhDvCerc"),
			},
			{
				ActivationEpoch:   xc.NewAmountBlockchainFromUint64(652),
				DeactivationEpoch: xc.NewAmountBlockchainFromUint64(18446744073709551615),
				AmountActive:      xc.NewAmountBlockchainFromUint64(10_000_000_000),
				AmountInactive:    xc.NewAmountBlockchainFromUint64(2282880),
				StakeAccount:      solana.MustPublicKeyFromBase58("6LFjBX1yUwSr8SWsyZUc5okZiVo8ZdmVQ9keJAazRmnh"),
			},
		},
	}

	tx, err := txBuilder.Unstake(args, input)
	require.NoError(t, err)
	require.NotNil(t, tx)

	deactivates := tx.(*Tx).GetDeactivateStakes()
	require.Len(t, deactivates, 3)
	require.Equal(t, input.EligibleStakes[0].StakeAccount, deactivates[0].Instruction.GetStakeAccount().PublicKey)
	require.Equal(t, input.EligibleStakes[1].StakeAccount, deactivates[1].Instruction.GetStakeAccount().PublicKey)
	require.Equal(t, input.EligibleStakes[2].StakeAccount, deactivates[2].Instruction.GetStakeAccount().PublicKey)

	splits := tx.(*Tx).GetSplitStakes()
	require.Len(t, splits, 1)
	// 5 SOL remainder to be split (along with the inactive stakes)
	require.EqualValues(t, 5_000_000_000+2282880*3, *splits[0].Instruction.Lamports)

}
func TestNewWithdrawTransfer(t *testing.T) {

	txBuilder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())

	from := xc.Address("83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH") // fails on parsing from
	validator := "J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp"

	amount := xc.NewAmountBlockchainFromUint64(10000000)
	args := buildertest.MustNewStakingArgs(xc.SOL, from, amount, xcbuilder.OptionValidator(validator))

	input := &tx_input.WithdrawInput{
		TxInput: tx_input.TxInput{
			RecentBlockHash:   solana.MustHashFromBase58("DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK"),
			PrioritizationFee: xc.NewAmountBlockchainFromUint64(100000),
		},
		EligibleStakes: []*tx_input.ExistingStake{
			{
				ActivationEpoch:   xc.NewAmountBlockchainFromUint64(649),
				DeactivationEpoch: xc.NewAmountBlockchainFromUint64(650),
				AmountActive:      xc.NewAmountBlockchainFromUint64(0),
				AmountInactive:    xc.NewAmountBlockchainFromUint64(3000280),
				StakeAccount:      solana.MustPublicKeyFromBase58("8zrSGLMdE6dK57Q7a8N8TDohmyft1MrsLYdRqhDvCerc"),
			},
			{
				ActivationEpoch:   xc.NewAmountBlockchainFromUint64(650),
				DeactivationEpoch: xc.NewAmountBlockchainFromUint64(650),
				AmountActive:      xc.NewAmountBlockchainFromUint64(0),
				AmountInactive:    xc.NewAmountBlockchainFromUint64(10000000),
				StakeAccount:      solana.MustPublicKeyFromBase58("6LFjBX1yUwSr8SWsyZUc5okZiVo8ZdmVQ9keJAazRmnh"),
			},
		},
	}

	tx, err := txBuilder.Withdraw(args, input)
	require.NoError(t, err)
	require.NotNil(t, tx)

	withdrawals := tx.(*Tx).GetStakeWithdraws()
	require.Len(t, withdrawals, 2)
	require.Equal(t, input.EligibleStakes[0].StakeAccount, withdrawals[0].Instruction.GetStakeAccount().PublicKey)
	require.Equal(t, input.EligibleStakes[1].StakeAccount, withdrawals[1].Instruction.GetStakeAccount().PublicKey)
	total := (*withdrawals[0].Instruction.Lamports) + (*withdrawals[1].Instruction.Lamports)
	fmt.Println(amount.Uint64())
	fmt.Println(total)
	require.EqualValues(t, amount.Uint64(), total)
}
