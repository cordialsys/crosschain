package client

import (
	"context"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
)

// Client is a client that can fetch data and submit tx to a public blockchain
type Client interface {
	// Fetch the basic transaction input for any new transaction
	FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error)

	// Broadcast a signed transaction to the chain
	SubmitTx(ctx context.Context, tx xc.Tx) error

	// Fetching transaction info - legacy endpoint
	FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error)

	// Fetching transaction info
	FetchTxInfo(ctx context.Context, txHash xc.TxHash) (TxInfo, error)

	// Fetch the balance of the given asset that the client is configured with
	FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error)

	// Fetch the native balance for the chain on an address
	FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error)

	// Fetch the precision (or "decimals") associated with the target asset
	FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error)
}
type ClientV2 interface {
	// Improved signature replacement of `FetchLegacyTxInput`
	FetchTransferInput(ctx context.Context, args builder.TransferArgs) (xc.TxInput, error)
}
type ClientWithDecimals interface {
	FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error)
}

type FullClient interface {
	Client
	ClientV2
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

// Special 3rd-party interface for Ethereum as ethereum doesn't understand delegated staking
type ManualUnstakingClient interface {
	CompleteManualUnstaking(ctx context.Context, unstake *Unstake) error
}

type ClientError string

// A transaction terminally failed due to no balance
const NoBalance ClientError = "NoBalance"

// A transaction terminally failed due to no balance after accounting for gas cost
const NoBalanceForGas ClientError = "NoBalanceForGas"

// A transaction terminally failed due to another reason
const TransactionFailure ClientError = "TransactionFailure"

// A transaction failed to submit because it already exists
const TransactionExists ClientError = "TransactionExists"

// deadline exceeded and transaction can no longer be accepted
const TransactionTimedOut ClientError = "TransactionTimedOut"

// A network error occured -- there may be nothing wrong with the transaction
const NetworkError ClientError = "NetworkError"

// No outcome for this error known
const UnknownError ClientError = "UnknownError"

type State string

var Activating State = "activating"
var Active State = "active"
var Deactivating State = "deactivating"
var Inactive State = "inactive"

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

func NewStakedBalance(balance xc.AmountBlockchain, state State, validator, account string) *StakedBalance {
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
