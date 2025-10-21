package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	buildererrors "github.com/cordialsys/crosschain/builder/errors"
	"github.com/cordialsys/crosschain/chain/solana/builder"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/cordialsys/crosschain/chain/solana/types"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/sirupsen/logrus"
)

type parsedStakeAccount struct {
	Account      *rpc.KeyedAccount
	StakeAccount types.StakeAccount
}

func (client *Client) GetStakeAccounts(ctx context.Context, address xc.Address) ([]*parsedStakeAccount, error) {
	stakeAuthority, err := solana.PublicKeyFromBase58(string(address))
	if err != nil {
		return nil, err
	}
	res, err := client.SolClient.GetProgramAccountsWithOpts(ctx, solana.StakeProgramID, &rpc.GetProgramAccountsOpts{
		Commitment: rpc.CommitmentFinalized,
		Encoding:   "jsonParsed",
		Filters: []rpc.RPCFilter{
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: 12,
					Bytes:  stakeAuthority[:],
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	var stakeAccounts []*parsedStakeAccount
	for _, acc := range res {
		var stakeAccount types.StakeAccount
		err := json.Unmarshal(acc.Account.Data.GetRawJSON(), &stakeAccount)
		if err != nil {
			return nil, err
		}
		stakeAccounts = append(stakeAccounts, &parsedStakeAccount{
			Account:      acc,
			StakeAccount: stakeAccount,
		})
	}
	return stakeAccounts, nil

}
func (client *Client) FetchStakeBalance(ctx context.Context, args xclient.StakedBalanceArgs) ([]*xclient.StakedBalance, error) {
	stakeAccounts, err := client.GetStakeAccounts(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}

	epochInfo, err := client.SolClient.GetEpochInfo(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, err
	}

	stakedBalances := []*xclient.StakedBalance{}

	for _, stake := range stakeAccounts {
		validator := stake.StakeAccount.Parsed.Info.Stake.Delegation.Voter
		account := stake.Account.Pubkey.String()

		inputValidator, ok := args.GetValidator()
		if ok {
			if inputValidator != validator {
				continue
			}
		}
		inputAccount, ok := args.GetAccount()
		if ok {
			if inputAccount != account {
				continue
			}
		}

		state := stake.StakeAccount.GetState(epochInfo.Epoch)
		stakedBalance := xclient.NewStakedBalance(
			xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.Stake),
			state,
			validator,
			account,
		)
		rentReserve := xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Meta.RentExemptReserve)
		// The rent-exempt reserve is added to the inactive balance
		stakedBalance.Balance.Inactive = stakedBalance.Balance.Inactive.Add(
			&rentReserve,
		)

		stakedBalances = append(stakedBalances, stakedBalance)
	}

	return stakedBalances, nil
}

func (client *Client) FetchStakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.StakeTxInput, error) {
	txInput, err := client.FetchBaseInput(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}
	// Set default fee for now
	txInput.PrioritizationFee = xc.NewAmountBlockchainFromUint64(100000)
	privKey, err := solana.NewRandomPrivateKey()
	if err != nil {
		return nil, err
	}
	stakeInput := tx_input.StakingInput{
		TxInput:    *txInput,
		StakingKey: privKey,
	}
	validatorAddress, ok := args.GetValidator()
	if !ok {
		return nil, errors.New("validator to be delegated to is required")
	}
	validatorPubkey, err := solana.PublicKeyFromBase58(string(validatorAddress))
	if err != nil {
		return nil, fmt.Errorf("invalid base58 for validator address: %v", err)
	}

	voteAccounts, err := client.SolClient.GetVoteAccounts(ctx, &rpc.GetVoteAccountsOpts{
		Commitment: rpc.CommitmentFinalized,
	})
	if err != nil {
		return nil, err
	}
	found := false
	for _, voteAccount := range voteAccounts.Current {
		if voteAccount.VotePubkey == validatorPubkey {
			stakeInput.ValidatorVoteAccount = voteAccount.VotePubkey
			found = true
			break
		}
		if voteAccount.NodePubkey == validatorPubkey {
			stakeInput.ValidatorVoteAccount = voteAccount.VotePubkey
			logrus.WithFields(logrus.Fields{
				"identity": voteAccount.NodePubkey.String(),
				"vote":     voteAccount.VotePubkey.String(),
			}).Warn("validator identity pubkey was input, using the vote pubkey instead")
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("validator vote account not found: %s", validatorAddress)
	}

	return &stakeInput, nil
}

func (client *Client) FetchUnstakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.UnstakeTxInput, error) {
	stakeAccounts, err := client.GetStakeAccounts(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}

	validator, ok := args.GetValidator()
	if !ok {
		return nil, fmt.Errorf("validator to be undelegated from is required")
	}
	_, err = solana.PublicKeyFromBase58(string(validator))
	if err != nil {
		return nil, fmt.Errorf("invalid base58 for validator address: %v", err)
	}
	a, ok := args.GetAmount()
	if !ok {
		return nil, buildererrors.ErrStakingAmountRequired
	}
	amount := a.Uint64()
	if amount < builder.RentExemptLamportsThreshold {
		return nil, fmt.Errorf("amount to unstake is below the rent exempt threshold (%s SOL)", builder.RentExemptLamportsThresholdHuman)
	}

	txInput, err := client.FetchBaseInput(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}
	// Set default fee for now
	txInput.PrioritizationFee = xc.NewAmountBlockchainFromUint64(100000)
	privKey, err := solana.NewRandomPrivateKey()
	if err != nil {
		return nil, err
	}
	epochInfo, err := client.SolClient.GetEpochInfo(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, err
	}

	matchingStakeAccounts := []*tx_input.ExistingStake{}
	for _, stake := range stakeAccounts {
		inputAccount, ok := args.GetStakeAccount()
		if ok {
			if stake.Account.Pubkey.String() != inputAccount {
				continue
			}
		}
		state := stake.StakeAccount.GetState(epochInfo.Epoch)
		if state == xclient.Deactivating || state == xclient.Inactive {
			// Skip unstaking for inactive or deactivating stakes
			continue
		}

		if stake.StakeAccount.Parsed.Info.Stake.Delegation.Voter == validator {
			amountStake := xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.Stake)
			amountRentReserve := xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Meta.RentExemptReserve)

			matchingStakeAccounts = append(matchingStakeAccounts, &tx_input.ExistingStake{
				ActivationEpoch:   xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.ActivationEpoch),
				DeactivationEpoch: xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.DeactivationEpoch),
				AmountActive:      amountStake,
				AmountInactive:    amountRentReserve,
				StakeAccount:      stake.Account.Pubkey,
			})
		}
	}
	sort.Slice(matchingStakeAccounts, func(i, j int) bool {
		// Sort in order by activation epoch, so that activated stakes are unstaked first
		return matchingStakeAccounts[i].ActivationEpoch.Uint64() < matchingStakeAccounts[j].ActivationEpoch.Uint64()
	})
	if len(matchingStakeAccounts) == 0 {
		return nil, fmt.Errorf("no activating or active stake accounts found for validator: %s", validator)
	}
	unstakeInput := tx_input.UnstakingInput{
		TxInput:        *txInput,
		StakingKey:     privKey,
		EligibleStakes: matchingStakeAccounts,
	}
	return &unstakeInput, nil
}

func (client *Client) FetchWithdrawInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.WithdrawTxInput, error) {
	stakeAccounts, err := client.GetStakeAccounts(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}
	txInput, err := client.FetchBaseInput(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}
	// Set default fee for now
	txInput.PrioritizationFee = xc.NewAmountBlockchainFromUint64(100000)
	epochInfo, err := client.SolClient.GetEpochInfo(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, err
	}

	matchingStakeAccounts := []*tx_input.ExistingStake{}
	for _, stake := range stakeAccounts {
		inputValidator, ok := args.GetValidator()
		if ok {
			if stake.StakeAccount.Parsed.Info.Stake.Delegation.Voter != inputValidator {
				continue
			}
		}
		inputAccount, ok := args.GetStakeAccount()
		if ok {
			if stake.Account.Pubkey.String() != inputAccount {
				continue
			}
		}
		state := stake.StakeAccount.GetState(epochInfo.Epoch)
		if state != xclient.Inactive {
			// only able to withdraw inactive balances
			continue
		}

		amountStake := xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.Stake)
		amountRentReserve := xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Meta.RentExemptReserve)
		matchingStakeAccounts = append(matchingStakeAccounts, &tx_input.ExistingStake{
			ActivationEpoch:   xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.ActivationEpoch),
			DeactivationEpoch: xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.DeactivationEpoch),
			AmountActive:      xc.NewAmountBlockchainFromUint64(0),
			AmountInactive:    amountStake.Add(&amountRentReserve),
			StakeAccount:      stake.Account.Pubkey,
		})
	}
	sort.Slice(matchingStakeAccounts, func(i, j int) bool {
		// Sort in order by activation epoch, so that activated stakes are withdrawn first
		return matchingStakeAccounts[i].ActivationEpoch.Uint64() < matchingStakeAccounts[j].ActivationEpoch.Uint64()
	})
	withdrawInput := tx_input.WithdrawInput{
		TxInput:        *txInput,
		EligibleStakes: matchingStakeAccounts,
	}
	return &withdrawInput, nil
}
