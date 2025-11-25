package client

import (
	"context"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/cordialsys/crosschain/client/types"
)

// Client is a client that can fetch data and submit tx to a public blockchain
type Client interface {
	// Fetch transaction input for a transfer
	FetchTransferInput(ctx context.Context, args builder.TransferArgs) (xc.TxInput, error)

	// Broadcast a signed transaction to the chain
	SubmitTx(ctx context.Context, submitRex types.SubmitTxReq) error

	// Fetching transaction info - legacy endpoint
	FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error)

	// Fetching transaction info
	FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error)

	// Fetch the balance of the given asset that the client is configured with
	FetchBalance(ctx context.Context, args *BalanceArgs) (xc.AmountBlockchain, error)

	// Fetch the precision (or "decimals") associated with the target asset
	FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error)

	// Fetch a specific block or the latest block
	FetchBlock(ctx context.Context, args *BlockArgs) (*txinfo.BlockWithTransactions, error)
}

type MultiTransferClient interface {
	FetchMultiTransferInput(ctx context.Context, args builder.MultiTransferArgs) (xc.MultiTransferInput, error)
}

type StakingClient interface {
	// Fetch staked balances accross different possible states
	FetchStakeBalance(ctx context.Context, args StakedBalanceArgs) ([]*StakedBalance, error)

	// Fetch inputs required for a staking transaction
	FetchStakingInput(ctx context.Context, args builder.StakeArgs) (xc.StakeTxInput, error)

	// Fetch inputs required for a unstaking transaction
	FetchUnstakingInput(ctx context.Context, args builder.StakeArgs) (xc.UnstakeTxInput, error)

	// Fetch input for a withdraw transaction -- not all chains use this as they combine it with unstake
	FetchWithdrawInput(ctx context.Context, args builder.StakeArgs) (xc.WithdrawTxInput, error)
}

type CallClient interface {
	// Fetch inputs required for a call transaction
	FetchCallInput(ctx context.Context, call xc.TxCall) (xc.CallTxInput, error)
}

// Special 3rd-party interface for Ethereum as ethereum doesn't understand delegated staking
type ManualUnstakingClient interface {
	CompleteManualUnstaking(ctx context.Context, unstake *txinfo.Unstake) error
}

type StakeState string

var Activating StakeState = "activating"
var Active StakeState = "active"
var Deactivating StakeState = "deactivating"
var Inactive StakeState = "inactive"

type StakedBalanceState struct {
	Active       xc.AmountBlockchain `json:"active,omitempty"`
	Activating   xc.AmountBlockchain `json:"activating,omitempty"`
	Deactivating xc.AmountBlockchain `json:"deactivating,omitempty"`
	Inactive     xc.AmountBlockchain `json:"inactive,omitempty"`
}

type StakedBalance struct {
	// the validator that the stake is delegated to
	Validator string `json:"validator"`
	// Optional; the account that the stake is associated with
	Account string `json:"account,omitempty"`
	// The states balance of the balance in the validator [+account]
	Balance StakedBalanceState `json:"balance"`
}

func NewStakedBalances(balances StakedBalanceState, validator, account string) *StakedBalance {
	return &StakedBalance{
		Validator: validator,
		Account:   account,
		Balance:   balances,
	}
}

func NewStakedBalance(balance xc.AmountBlockchain, state StakeState, validator, account string) *StakedBalance {
	balances := StakedBalanceState{}
	switch state {
	case Activating:
		balances.Activating = balance
	case Active:
		balances.Active = balance
	case Deactivating:
		balances.Deactivating = balance
	case Inactive:
		balances.Inactive = balance
	}
	return &StakedBalance{
		Validator: validator,
		Account:   account,
		Balance:   balances,
	}
}
