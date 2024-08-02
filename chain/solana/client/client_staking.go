package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
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
		active := uint64(0)
		inactive := uint64(0)
		activating := uint64(0)
		deactivating := uint64(0)
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

		// if stake.StakeAccount.Parsed.Info.Stake.Delegation.Voter == validator {
		activationEpoch := xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.ActivationEpoch).Uint64()
		deactivationEpoch := xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.DeactivationEpoch).Uint64()

		if deactivationEpoch == epochInfo.Epoch {
			// deactivation is occuring
			deactivating += xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.Stake).Uint64()
		} else if deactivationEpoch < epochInfo.Epoch {
			// deactivation occured
			inactive += xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.Stake).Uint64()
		} else if activationEpoch < epochInfo.Epoch {
			// activation occured
			active += xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.Stake).Uint64()
		} else {
			// must be activating
			activating += xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.Stake).Uint64()
		}

		// The rent-exempt reserve is added to the inactive balance
		inactive += xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Meta.RentExemptReserve).Uint64()
		// }
		stakedBalances = append(stakedBalances, xclient.NewStakedBalances(xclient.StakedBalanceState{
			Activating:   xc.NewAmountBlockchainFromUint64(activating),
			Active:       xc.NewAmountBlockchainFromUint64(active),
			Deactivating: xc.NewAmountBlockchainFromUint64(deactivating),
			Inactive:     xc.NewAmountBlockchainFromUint64(inactive),
		}, validator, account))
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

	matchingStakeAccounts := []*tx_input.ExistingStake{}
	for _, stake := range stakeAccounts {
		if stake.StakeAccount.Parsed.Info.Stake.Delegation.Voter == validator {
			matchingStakeAccounts = append(matchingStakeAccounts, &tx_input.ExistingStake{
				ActivationEpoch:      xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.ActivationEpoch).Uint64(),
				DeactivationEpoch:    xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.DeactivationEpoch).Uint64(),
				Amount:               xc.NewAmountBlockchainFromStr(stake.StakeAccount.Parsed.Info.Stake.Delegation.Stake),
				ValidatorVoteAccount: stake.StakeAccount.Parsed.Info.Stake.Delegation.Voter,
				StakeAccount:         stake.Account.Pubkey.String(),
			})
		}
	}
	unstakeInput := tx_input.UnstakingInput{
		TxInput:        *txInput,
		StakingKey:     privKey,
		ExistingStakes: matchingStakeAccounts,
	}
	return &unstakeInput, nil
}
