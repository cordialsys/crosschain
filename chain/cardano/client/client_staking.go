package client

import (
	"context"
	"fmt"
	"strconv"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	buildererrors "github.com/cordialsys/crosschain/builder/errors"
	"github.com/cordialsys/crosschain/chain/cardano/address"
	clienterrors "github.com/cordialsys/crosschain/chain/cardano/client/errors"
	"github.com/cordialsys/crosschain/chain/cardano/client/types"
	"github.com/cordialsys/crosschain/chain/cardano/tx"
	"github.com/cordialsys/crosschain/chain/cardano/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
)

var _ xclient.StakingClient = &Client{}

func (c *Client) FetchStakeBalance(ctx context.Context, args xclient.StakedBalanceArgs) ([]*xclient.StakedBalance, error) {
	path := fmt.Sprintf("/%s/%s", EndpointAddresses, string(args.GetFrom()))
	var getAddressInfoResponse types.GetAddressInfoResponse
	err := c.Get(ctx, path, &getAddressInfoResponse)
	if err != nil {
		return nil, clienterrors.AddressInfof(err)
	}

	var active xc.AmountBlockchain
	for _, amount := range getAddressInfoResponse.Amounts {
		if amount.Unit == types.Lovelace {
			active = xc.NewAmountBlockchainFromStr(amount.Quantity)
		}
	}

	rewardsPath := fmt.Sprintf("/%s/%s", EndpointAccounts, getAddressInfoResponse.StakeAddress)
	var getAccountInfoResponse types.GetAccountInfoResponse
	err = c.Get(ctx, rewardsPath, &getAccountInfoResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account info: %w", err)
	}

	balances := make([]*xclient.StakedBalance, 0)
	if getAccountInfoResponse.PoolId != "" {
		stakedBalance := xclient.NewStakedBalance(
			active,
			xclient.Active,
			getAccountInfoResponse.PoolId,
			"",
		)
		stakedBalance.Balance.Inactive = xc.NewAmountBlockchainFromStr(getAccountInfoResponse.WithdrawableAmount)
		balances = append(balances, stakedBalance)
	}
	return balances, nil
}

func (c *Client) FetchStakingInput(ctx context.Context, args builder.StakeArgs) (xc.StakeTxInput, error) {
	_, ok := args.GetAmount()
	if ok {
		return nil, buildererrors.ErrStakingAmountNotUsed
	}
	protocolParams, err := c.FetchProtocolParameters(ctx)
	if err != nil {
		return nil, clienterrors.ProtocolParamsf(err)
	}

	da, err := strconv.ParseUint(protocolParams.KeyDeposit, 10, 64)
	if err != nil {
		return nil, clienterrors.DepositValuef(err)
	}
	depositAmount := xc.NewAmountBlockchainFromUint64(da)
	contract := xc.ContractAddress(types.Lovelace)
	baseInput, err := c.fetchBaseInput(
		ctx,
		depositAmount,
		contract,
		args.GetFrom(),
		protocolParams,
	)
	if err != nil {
		return nil, clienterrors.BaseInputf(err)
	}

	stakeInput := tx_input.StakingInput{
		TxInput:    *baseInput,
		KeyDeposit: depositAmount.Uint64(),
	}
	transaction, err := tx.NewStake(args, &stakeInput)
	if err != nil {
		return nil, clienterrors.FeeEstimationf(err)
	}

	// staking requires 2 signatures
	err = transaction.SetSignatures([]*xc.SignatureResponse{
		{
			Signature: make([]byte, 64),
			PublicKey: make([]byte, 32),
		},
		{
			Signature: make([]byte, 64),
			PublicKey: make([]byte, 32),
		},
	}...)
	if err != nil {
		return nil, fmt.Errorf("failed to set signatures: %w", err)
	}

	err = stakeInput.CalculateTxFee(transaction)
	if err != nil {
		return nil, clienterrors.CalculateTxFee(err)
	}

	return &stakeInput, nil
}

// Fetch inputs required for a unstaking transaction
func (c *Client) FetchUnstakingInput(ctx context.Context, args builder.StakeArgs) (xc.UnstakeTxInput, error) {
	_, ok := args.GetAmount()
	if ok {
		return nil, buildererrors.ErrStakingAmountNotUsed
	}

	pubkey, ok := args.GetPublicKey()
	if !ok {
		return nil, fmt.Errorf("cardano unstaking require a valid pubkey")
	}

	validator, ok := args.GetValidator()
	if !ok {
		return nil, buildererrors.ErrValidatorRequired
	}

	stakeAddress, err := address.GetStakeAddress(pubkey, c.IsMainnet())
	if err != nil {
		return nil, fmt.Errorf("failed to get rewards address: %w", err)
	}

	accountsPath := fmt.Sprintf("/%s/%s", EndpointAccounts, stakeAddress)
	var getAccountInfoResponse types.GetAccountInfoResponse
	err = c.Get(ctx, accountsPath, &getAccountInfoResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account info: %w", err)
	}
	if getAccountInfoResponse.PoolId == "" {
		return nil, fmt.Errorf("cannot unstake: pool id is already empty")
	}
	if getAccountInfoResponse.PoolId != validator {
		return nil, fmt.Errorf("cannot unstake: specified validator differs from active pool")
	}

	protocolParams, err := c.FetchProtocolParameters(ctx)
	if err != nil {
		return nil, clienterrors.ProtocolParamsf(err)
	}

	da, err := strconv.ParseUint(protocolParams.KeyDeposit, 10, 64)
	if err != nil {
		return nil, clienterrors.DepositValuef(err)
	}
	depositAmount := xc.NewAmountBlockchainFromUint64(da)
	contract := xc.ContractAddress(types.Lovelace)
	baseInput, err := c.fetchBaseInput(
		ctx,
		depositAmount,
		contract,
		args.GetFrom(),
		protocolParams,
	)
	if err != nil {
		return nil, clienterrors.BaseInputf(err)
	}

	unstakeInput := tx_input.UnstakingInput{
		TxInput:    *baseInput,
		KeyDeposit: depositAmount.Uint64(),
	}
	transaction, err := tx.NewUnstake(args, &unstakeInput)
	if err != nil {
		return nil, clienterrors.FeeEstimationf(err)
	}

	// unstaking requires 2 signatures
	err = transaction.SetSignatures([]*xc.SignatureResponse{
		{
			Signature: make([]byte, 64),
			PublicKey: make([]byte, 32),
		},
		{
			Signature: make([]byte, 64),
			PublicKey: make([]byte, 32),
		},
	}...)
	if err != nil {
		return nil, fmt.Errorf("failed to set signatures: %w", err)
	}

	err = unstakeInput.CalculateTxFee(transaction)
	if err != nil {
		return nil, clienterrors.CalculateTxFee(err)
	}

	return &unstakeInput, nil
}

// Fetch input for a withdraw transaction -- not all chains use this as they combine it with unstake
func (c *Client) FetchWithdrawInput(ctx context.Context, args builder.StakeArgs) (xc.WithdrawTxInput, error) {
	_, ok := args.GetAmount()
	if ok {
		return nil, buildererrors.ErrStakingAmountNotUsed
	}
	pubkey, ok := args.GetPublicKey()
	if !ok {
		return nil, fmt.Errorf("cardano withdrawals require a valid pubkey")
	}

	rewardsAddress, err := address.GetStakeAddress(pubkey, c.IsMainnet())
	if err != nil {
		return nil, fmt.Errorf("failed to get rewards address: %w", err)
	}

	contract := xc.ContractAddress(types.Lovelace)
	protocolParams, err := c.FetchProtocolParameters(ctx)
	if err != nil {
		return nil, clienterrors.ProtocolParamsf(err)
	}

	// our utxo input should cover gas fee and thats it
	baseInput, err := c.fetchBaseInput(
		ctx,
		xc.NewAmountBlockchainFromUint64(0),
		contract,
		args.GetFrom(),
		protocolParams,
	)
	if err != nil {
		return nil, clienterrors.BaseInputf(err)
	}

	rewardsPath := fmt.Sprintf("/accounts/%s", rewardsAddress)
	var getAccountInfoResponse types.GetAccountInfoResponse
	err = c.Get(ctx, rewardsPath, &getAccountInfoResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account info: %w", err)
	}

	withdrawAmount := xc.NewAmountBlockchainFromStr(getAccountInfoResponse.WithdrawableAmount)
	withdrawInput := &tx_input.WithdrawInput{
		TxInput:        *baseInput,
		RewardsAddress: rewardsAddress,
		RewardsAmount:  withdrawAmount,
	}
	transaction, err := tx.NewWithdraw(args, withdrawInput)
	if err != nil {
		return nil, clienterrors.FeeEstimationf(err)
	}

	// withdrawal requires 2 signatures
	err = transaction.SetSignatures([]*xc.SignatureResponse{
		{
			Signature: make([]byte, 64),
			PublicKey: make([]byte, 32),
		},
		{
			Signature: make([]byte, 64),
			PublicKey: make([]byte, 32),
		},
	}...)
	if err != nil {
		return nil, fmt.Errorf("failed to set signatures: %w", err)
	}

	err = withdrawInput.CalculateTxFee(transaction)
	if err != nil {
		return nil, clienterrors.CalculateTxFee(err)
	}

	return withdrawInput, nil
}
