package tron

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
	"github.com/golang/protobuf/ptypes"

	"github.com/cordialsys/crosschain/chain/tron/common"
	"github.com/cordialsys/crosschain/chain/tron/core"
	httpclient "github.com/cordialsys/crosschain/chain/tron/http_client"
	"github.com/cordialsys/crosschain/chain/tron/txinput"
	"golang.org/x/crypto/sha3"

	xcbuilder "github.com/cordialsys/crosschain/builder"
	buildererrors "github.com/cordialsys/crosschain/builder/errors"
	eABI "github.com/ethereum/go-ethereum/accounts/abi"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
)

type UnstakeVoteAction string

const (
	VoteForSpecificValidator UnstakeVoteAction = "validator"
	VoteForAnyValidator      UnstakeVoteAction = "any"
	NoVoteRequired           UnstakeVoteAction = "no_vote"
)

var ErrVotingInputRequired error = errors.New("voting required but no vote input provided")
var ErrFreezeInputRequired error = errors.New("freezing required but no vote input provided")

// TxBuilder for Template
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = &TxBuilder{}
var _ xcbuilder.Staking = &TxBuilder{}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

func GetAddressHash(address string) ([]byte, error) {
	to, v, err := base58.CheckDecode(address)
	if err != nil {
		return nil, err
	}
	var bs []byte
	bs = append(bs, v)
	bs = append(bs, to...)
	return bs, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {

	if contract, ok := args.GetContract(); ok {
		return txBuilder.NewTokenTransfer(args.GetFrom(), args.GetTo(), args.GetAmount(), contract, input)
	} else {
		return txBuilder.NewNativeTransfer(args.GetFrom(), args.GetTo(), args.GetAmount(), input)
	}
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	from_bytes, err := GetAddressHash(string(from))
	if err != nil {
		return nil, err
	}
	to_bytes, err := GetAddressHash(string(to))
	if err != nil {
		return nil, err
	}
	params := &core.TransferContract{}
	params.Amount = amount.Int().Int64()
	params.OwnerAddress = from_bytes
	params.ToAddress = to_bytes

	contract := &core.Transaction_Contract{}
	contract.Type = core.Transaction_Contract_TransferContract
	param, err := ptypes.MarshalAny(params)
	if err != nil {
		return nil, err
	}
	contract.Parameter = param

	i := input.(*txinput.TxInput)
	tx := new(core.Transaction)
	tx.RawData = i.ToRawData(contract)

	txWrapper := NewTx()
	txWrapper.AppendTx(tx)
	return txWrapper, nil
}

// Signature of a method
func Signature(method string) []byte {
	// hash method
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write([]byte(method))
	b := hasher.Sum(nil)
	return b[:4]
}

// NewTokenTransfer creates a new transfer for a token asset
func (txBuilder TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, contract xc.ContractAddress, input xc.TxInput) (xc.Tx, error) {
	from_bytes, err := GetAddressHash(string(from))
	if err != nil {
		return nil, fmt.Errorf("invalid from address: %v", err)
	}

	to_bytes, err := GetAddressHash(string(to))
	if err != nil {
		return nil, fmt.Errorf("invalid to address: %v", err)
	}

	contract_bytes, err := GetAddressHash(string(contract))
	if err != nil {
		return nil, fmt.Errorf("invalid contract address: %v", err)
	}

	addrType, err := eABI.NewType("address", "", nil)
	if err != nil {
		return nil, fmt.Errorf("internal type construction error: %v", err)
	}
	amountType, err := eABI.NewType("uint256", "", nil)
	if err != nil {
		return nil, fmt.Errorf("internal type construction error: %v", err)
	}
	args := eABI.Arguments{
		{
			Type: addrType,
		},
		{
			Type: amountType,
		},
	}

	paramBz, err := args.PackValues([]interface{}{
		ethcommon.BytesToAddress(to_bytes),
		amount.Int(),
	})
	if err != nil {
		return nil, fmt.Errorf("could not pack: %v", err)
	}
	methodSig := Signature("transfer(address,uint256)")
	data := append(methodSig, paramBz...)

	params := &core.TriggerSmartContract{}
	params.ContractAddress = contract_bytes
	params.Data = data
	params.OwnerAddress = from_bytes
	params.CallValue = 0

	contractParam := &core.Transaction_Contract{}
	contractParam.Type = core.Transaction_Contract_TriggerSmartContract
	param, err := ptypes.MarshalAny(params)
	if err != nil {
		return nil, fmt.Errorf("could not marshal any: %v", err)
	}
	contractParam.Parameter = param

	i := input.(*txinput.TxInput)
	tx := &core.Transaction{}
	tx.RawData = i.ToRawData(contractParam)
	// set limit for token contracts
	tx.RawData.FeeLimit = int64(i.MaxFee.Uint64())
	if tx.RawData.FeeLimit == 0 {
		logrus.Warn("tron max-fee missing from tx-input")
		// 200 tron sanity limit
		tx.RawData.FeeLimit = 200_000_000
	}

	txWrapper := NewTx()
	txWrapper.AppendTx(tx)
	return txWrapper, nil
}

func (txBuilder TxBuilder) NewFreeze(from xc.Address, balance xc.AmountBlockchain, input xc.TxInput) (*core.Transaction, error) {
	from_bytes, err := GetAddressHash(string(from))
	if err != nil {
		return nil, err
	}

	contract := &core.FreezeBalanceV2Contract{}
	contract.OwnerAddress = from_bytes
	contract.FrozenBalance = balance.Int().Int64()
	contract.Resource = core.ResourceCode_BANDWIDTH

	params, err := ptypes.MarshalAny(contract)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal any params: %w", err)
	}

	tx_contract := &core.Transaction_Contract{
		Type:      core.Transaction_Contract_FreezeBalanceV2Contract,
		Parameter: params,
	}

	i := input.(*txinput.TxInput)
	tx := new(core.Transaction)
	tx.RawData = i.ToRawData(tx_contract)

	return tx, nil
}

func (txBuilder TxBuilder) NewUnfreeze(from xc.Address, balance xc.AmountBlockchain, input xc.TxInput) (*core.Transaction, error) {
	from_bytes, err := GetAddressHash(string(from))
	if err != nil {
		return nil, err
	}

	contract := &core.UnfreezeBalanceV2Contract{}
	contract.OwnerAddress = from_bytes
	contract.UnfreezeBalance = balance.Int().Int64()
	contract.Resource = core.ResourceCode_BANDWIDTH

	params, err := ptypes.MarshalAny(contract)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal any params: %w", err)
	}

	tx_contract := &core.Transaction_Contract{
		Type:      core.Transaction_Contract_UnfreezeBalanceV2Contract,
		Parameter: params,
	}

	i := input.(*txinput.TxInput)
	tx := new(core.Transaction)
	tx.RawData = i.ToRawData(tx_contract)

	return tx, nil
}

func (txBuilder TxBuilder) NewVotes(from xc.Address, votes []*httpclient.Vote, input *txinput.TxInput) (*core.Transaction, error) {
	from_bytes, err := GetAddressHash(string(from))
	if err != nil {
		return nil, err
	}

	contract := &core.VoteWitnessContract{}
	contract.OwnerAddress = from_bytes
	contract.Votes = make([]*core.VoteWitnessContract_Vote, len(votes))

	for i, v := range votes {
		addrhash, err := GetAddressHash(string(v.VoteAddress))
		if err != nil {
			return nil, fmt.Errorf("failed to get super representative address hash: %w", err)
		}
		contract.Votes[i] = &core.VoteWitnessContract_Vote{
			VoteAddress: addrhash,
			VoteCount:   int64(v.VoteCount),
		}
	}

	params, err := ptypes.MarshalAny(contract)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal any params: %w", err)
	}

	tx_contract := &core.Transaction_Contract{
		Type:      core.Transaction_Contract_VoteWitnessContract,
		Parameter: params,
	}

	tx := new(core.Transaction)
	tx.RawData = input.ToRawData(tx_contract)

	return tx, nil
}

func (txBuilder TxBuilder) Stake(stakingArgs xcbuilder.StakeArgs, input xc.StakeTxInput) (xc.Tx, error) {
	stakingInput, ok := input.(*txinput.StakeInput)
	if !ok {
		return nil, errors.New("invalid input type")
	}

	validator, ok := stakingArgs.GetValidator()
	if !ok {
		return nil, buildererrors.ErrValidatorRequired
	}

	stakeAmount, ok := stakingArgs.GetAmount()
	if !ok {
		return nil, buildererrors.ErrStakingAmountRequired
	}

	stakeVotes, err := common.TrxToVotes(stakeAmount, stakingInput.Decimals)
	if err != nil {
		return nil, ErrFailedTrxToVotesConversion
	}
	err = validateStakeAmount(stakeAmount, stakeVotes, stakingInput.Decimals)
	if err != nil {
		return nil, err
	}

	var validatorVotes *httpclient.Vote
	usedVotes := uint64(0)
	for _, v := range stakingInput.Votes {
		usedVotes += v.VoteCount
		if v.VoteAddress == validator {
			validatorVotes = v
		}
	}
	freezedBalance := xc.NewAmountBlockchainFromUint64(stakingInput.FreezedBalance)
	availableVotes, err := common.TrxToVotes(freezedBalance, stakingInput.Decimals)
	if err != nil {
		return nil, ErrFailedTrxToVotesConversion
	}
	unusedVotes := availableVotes - usedVotes

	from := stakingArgs.GetFrom()
	tx := NewTx()

	// check if we have to freeze additional TRX amount to satisfy stake requirements
	if unusedVotes < stakeVotes {
		if stakingInput.FreezeInput == nil {
			return nil, ErrFreezeInputRequired
		}
		missingVotes := stakeVotes - unusedVotes

		freezeBalance := common.VotesToTrx(missingVotes, stakingInput.Decimals)
		freeze, err := txBuilder.NewFreeze(from, freezeBalance, &stakingInput.TxInput)
		if err != nil {
			return nil, fmt.Errorf("failed to create freeze tx: %w", err)
		}
		tx.AppendTx(freeze)
	}

	// tron VoteWitnessContract requires a full list of votes
	// append a vote for the validator if we didn't vote for it in the past
	if validatorVotes == nil {
		stakingInput.Votes = append(stakingInput.Votes, &httpclient.Vote{
			VoteAddress: validator,
			VoteCount:   stakeVotes,
		})
	} else {
		validatorVotes.VoteCount += stakeVotes
	}
	votes, err := txBuilder.NewVotes(from, stakingInput.Votes, &stakingInput.TxInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create vote tx: %w", err)
	}

	tx.AppendTx(votes)

	return tx, nil
}

// Iterate through votes and remove exactly reduceCount votes
func ReduceTotalVoteCount(votes []*httpclient.Vote, reduceCount uint64) error {
	sort.Slice(votes, func(i, j int) bool {
		return votes[i].VoteCount < votes[j].VoteCount
	})
	for _, v := range votes {
		removedVotes := uint64(0)
		if v.VoteCount < reduceCount {
			removedVotes = v.VoteCount
		} else {
			removedVotes = reduceCount
		}
		reduceCount -= removedVotes
		v.VoteCount -= reduceCount

		if reduceCount == 0 {
			break
		}
	}

	if reduceCount > 0 {
		return errors.New("insufficient stake balance")
	}

	return nil
}

type UnstakeVoteDecision struct {
	Action         UnstakeVoteAction
	ValidatorVotes *httpclient.Vote
}

// we have to create a new vote tx in two cases:
// 1. user requested to unstake from a specific validator (super representative)
// 2. remaining votes are not sufficient to cover unstake amount
func DetermineUnstakeVoteAction(input *txinput.UnstakeInput, validator string, votesToUnstake uint64) (UnstakeVoteDecision, error) {
	var validatorVotes *httpclient.Vote
	usedVotes := uint64(0)
	for _, v := range input.Votes {
		usedVotes += v.VoteCount
		if validator != "" && validator == v.VoteAddress {
			validatorVotes = v
		}
	}

	if validator != "" && validatorVotes == nil {
		return UnstakeVoteDecision{}, errors.New("cannot unstake from validator %s: no active votes for this validator")
	}

	if validatorVotes != nil {
		if input.VoteInput == nil {
			return UnstakeVoteDecision{}, ErrVotingInputRequired
		}

		if validatorVotes.VoteCount < votesToUnstake {
			return UnstakeVoteDecision{}, fmt.Errorf(
				"not enought votes on validator: %s, required: %d, got: %d",
				validator,
				votesToUnstake,
				validatorVotes.VoteCount,
			)
		}

		return UnstakeVoteDecision{
			Action:         VoteForSpecificValidator,
			ValidatorVotes: validatorVotes,
		}, nil
	}

	freezedBalance := xc.NewAmountBlockchainFromUint64(input.FreezedBalance)
	totalVotes, err := common.TrxToVotes(freezedBalance, input.Decimals)
	if err != nil {
		return UnstakeVoteDecision{}, ErrFailedTrxToVotesConversion
	}
	remainingVotes := totalVotes - usedVotes
	if remainingVotes < votesToUnstake {
		if input.VoteInput == nil {
			return UnstakeVoteDecision{}, ErrVotingInputRequired
		}
		return UnstakeVoteDecision{
			Action: VoteForAnyValidator,
		}, nil
	} else {
		return UnstakeVoteDecision{
			Action: NoVoteRequired,
		}, nil
	}
}

func (txBuilder TxBuilder) Unstake(stakingArgs xcbuilder.StakeArgs, input xc.UnstakeTxInput) (xc.Tx, error) {
	stakingInput, ok := input.(*txinput.UnstakeInput)
	if !ok {
		return nil, errors.New("invalid input type")
	}
	stakeAmount, ok := stakingArgs.GetAmount()
	if !ok {
		return nil, buildererrors.ErrStakingAmountRequired
	}

	stakeVotes, err := common.TrxToVotes(stakeAmount, stakingInput.Decimals)
	if err != nil {
		return nil, ErrFailedTrxToVotesConversion
	}

	err = validateStakeAmount(stakeAmount, stakeVotes, stakingInput.Decimals)
	if err != nil {
		return nil, err
	}

	from := stakingArgs.GetFrom()
	tx := NewTx()

	validator, _ := stakingArgs.GetValidator()
	voteDecision, err := DetermineUnstakeVoteAction(stakingInput, validator, stakeVotes)
	if err != nil {
		return nil, fmt.Errorf("failed to determine vote action: %w", err)
	}

	switch voteDecision.Action {
	case VoteForSpecificValidator:
		if voteDecision.ValidatorVotes == nil {
			return nil, errors.New("expected to unstake from validator, but votes were not provided")
		}
		voteDecision.ValidatorVotes.VoteCount -= stakeVotes
		votes, err := txBuilder.NewVotes(from, stakingInput.Votes, stakingInput.VoteInput)
		if err != nil {
			return nil, fmt.Errorf("failed to create vote tx: %w", err)
		}
		tx.AppendTx(votes)
	case VoteForAnyValidator:
		err = ReduceTotalVoteCount(stakingInput.Votes, stakeVotes)
		if err != nil {
			return nil, fmt.Errorf("failed to unstake votes: %w", err)
		}
		votes, err := txBuilder.NewVotes(from, stakingInput.Votes, stakingInput.VoteInput)
		if err != nil {
			return nil, fmt.Errorf("failed to create vote tx: %w", err)
		}
		tx.AppendTx(votes)
	case NoVoteRequired:
		// skip
	}

	unfreeze, err := txBuilder.NewUnfreeze(from, stakeAmount, &stakingInput.TxInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create unfreeze transaction: %w", err)
	}
	tx.AppendTx(unfreeze)

	return tx, nil
}

func (txBuilder TxBuilder) Withdraw(stakingArgs xcbuilder.StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error) {
	withdrawInput, ok := input.(*txinput.WithdrawInput)
	if !ok {
		return nil, errors.New("invalid input type")
	}

	txWrapper := NewTx()
	if withdrawInput.TxInput != nil {
		from_bytes, err := GetAddressHash(string(stakingArgs.GetFrom()))
		if err != nil {
			return nil, err
		}

		contract := &core.WithdrawExpireUnfreezeContract{}
		contract.OwnerAddress = from_bytes

		params, err := ptypes.MarshalAny(contract)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal any params: %w", err)
		}

		tx_contract := &core.Transaction_Contract{
			Type:      core.Transaction_Contract_WithdrawExpireUnfreezeContract,
			Parameter: params,
		}

		tx := new(core.Transaction)
		tx.RawData = withdrawInput.TxInput.ToRawData(tx_contract)
		txWrapper.AppendTx(tx)
	}

	if withdrawInput.WithdrawRewardsInput != nil {
		from_bytes, err := GetAddressHash(string(stakingArgs.GetFrom()))
		if err != nil {
			return nil, err
		}

		contract := &core.WithdrawBalanceContract{}
		contract.OwnerAddress = from_bytes

		params, err := ptypes.MarshalAny(contract)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal any params: %w", err)
		}

		tx_contract := &core.Transaction_Contract{
			Type:      core.Transaction_Contract_WithdrawBalanceContract,
			Parameter: params,
		}

		tx := new(core.Transaction)
		tx.RawData = withdrawInput.WithdrawRewardsInput.ToRawData(tx_contract)
		txWrapper.AppendTx(tx)

	}

	if len(txWrapper.TronTxs) == 0 {
		return nil, errors.New("no rewards to withdraw")
	}

	return txWrapper, nil
}

// check that input is following 1 vote == 1 trx logic
// 1. make sure that stake amount is >= to vote amount
// 2. make sure that stakeAmount - voteTrxAmount is no greater than 1 TRX
func validateStakeAmount(argsAmount xc.AmountBlockchain, votes uint64, decimals int) error {
	decimalMultiplier := math.Pow10(decimals)
	votesAmount := uint64(decimalMultiplier * float64(votes))
	xcVotesAmount := xc.NewAmountBlockchainFromUint64(votesAmount)

	if argsAmount.Cmp(&xcVotesAmount) == -1 {
		return errors.New("stake amount is lesser than vote amount in trx")
	}

	one := xc.NewAmountHumanReadableFromFloat(1.0)
	bcOne := one.ToBlockchain(int32(decimals))
	amountDiff := argsAmount.Sub(&xcVotesAmount)
	if amountDiff.Cmp(&bcOne) > 0 {
		return errors.New("difference between requested amount and input vote amount is too big")
	}

	return nil
}
