package client

import (
	"context"
	"fmt"
	"strconv"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/cardano/address"
	clienterrors "github.com/cordialsys/crosschain/chain/cardano/client/errors"
	"github.com/cordialsys/crosschain/chain/cardano/client/types"
	"github.com/cordialsys/crosschain/chain/cardano/tx"
	"github.com/cordialsys/crosschain/chain/cardano/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
)

var _ xclient.StakingClient = &Client{}

func (c *Client) FetchStakeBalance(ctx context.Context, args xclient.StakedBalanceArgs) ([]*xclient.StakedBalance, error) {
	path := fmt.Sprintf("/addresses/%s", string(args.GetFrom()))
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

	rewardsPath := fmt.Sprintf("/accounts/%s", getAddressInfoResponse.StakeAddress)
	var getAccountInfoResponse types.GetAccountInfoResponse
	err = c.Get(ctx, rewardsPath, &getAccountInfoResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account info: %w", err)
	}

	stakedBalance := xclient.NewStakedBalance(
		active,
		xclient.Active,
		getAccountInfoResponse.PoolId,
		"",
	)
	stakedBalance.Balance.Inactive = xc.NewAmountBlockchainFromStr(getAccountInfoResponse.WithdrawableAmount)
	return []*xclient.StakedBalance{stakedBalance}, nil
}

func (c *Client) FetchStakingInput(ctx context.Context, args builder.StakeArgs) (xc.StakeTxInput, error) {
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
	transaction.SetSignatures([]*xc.SignatureResponse{
		{
			Signature: make([]byte, 64),
			PublicKey: make([]byte, 32),
		},
		{
			Signature: make([]byte, 64),
			PublicKey: make([]byte, 32),
		},
	}...)

	err = stakeInput.CalculateTxFee(transaction)
	if err != nil {
		return nil, clienterrors.CalculateTxFee(err)
	}

	return &stakeInput, nil
}

// Fetch inputs required for a unstaking transaction
func (c *Client) FetchUnstakingInput(ctx context.Context, args builder.StakeArgs) (xc.UnstakeTxInput, error) {
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
	transaction.SetSignatures([]*xc.SignatureResponse{
		{
			Signature: make([]byte, 64),
			PublicKey: make([]byte, 32),
		},
		{
			Signature: make([]byte, 64),
			PublicKey: make([]byte, 32),
		},
	}...)

	err = unstakeInput.CalculateTxFee(transaction)
	if err != nil {
		return nil, clienterrors.CalculateTxFee(err)
	}

	return &unstakeInput, nil
}

// Fetch input for a withdraw transaction -- not all chains use this as they combine it with unstake
func (c *Client) FetchWithdrawInput(ctx context.Context, args builder.StakeArgs) (xc.WithdrawTxInput, error) {
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

	withdrawInput := &tx_input.WithdrawInput{
		TxInput:        *baseInput,
		RewardsAddress: rewardsAddress,
	}
	transaction, err := tx.NewWithdraw(args, withdrawInput)
	if err != nil {
		return nil, clienterrors.FeeEstimationf(err)
	}

	// withdrawal requires 2 signatures
	transaction.SetSignatures([]*xc.SignatureResponse{
		{
			Signature: make([]byte, 64),
			PublicKey: make([]byte, 32),
		},
		{
			Signature: make([]byte, 64),
			PublicKey: make([]byte, 32),
		},
	}...)

	err = withdrawInput.CalculateTxFee(transaction)
	if err != nil {
		return nil, clienterrors.CalculateTxFee(err)
	}

	return withdrawInput, nil
}
