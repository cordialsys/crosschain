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
}
type ClientV2 interface {
	// Improved signature replacement of `FetchLegacyTxInput`
	FetchTransferInput(ctx context.Context, args builder.TransferArgs) (xc.TxInput, error)
}

type FullClient interface {
	Client
	ClientV2
}

type StakingClient interface {
	// Fetch staked balances accross different possible states
	FetchStakeBalance(ctx context.Context, address xc.Address, validator string, stakeAccount xc.Address) ([]*StakedBalance, error)

	// Fetch inputs required for a staking transaction
	FetchStakingInput(ctx context.Context, args builder.StakeArgs) (xc.StakeTxInput, error)

	// Fetch inputs required for a unstaking transaction
	FetchUnstakingInput(ctx context.Context, args builder.StakeArgs) (xc.UnstakeTxInput, error)
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

type StakedBalance struct {
	State  State               `json:"state"`
	Amount xc.AmountBlockchain `json:"amount"`
	// the validator that the stake is delegated to
	Validator string `json:"validator,omitempty"`
}

type StakedBalances struct {
	Active       xc.AmountBlockchain `json:"active,omitempty"`
	Activating   xc.AmountBlockchain `json:"activating,omitempty"`
	Deactivating xc.AmountBlockchain `json:"deactivating,omitempty"`
	Inactive     xc.AmountBlockchain `json:"inactive,omitempty"`
}

func NewStakedBalances(validator string, balances *StakedBalances) []*StakedBalance {
	balancesList := []*StakedBalance{}
	if !balances.Activating.IsZero() {
		balancesList = append(balancesList, &StakedBalance{
			State:     Activating,
			Amount:    balances.Activating,
			Validator: validator,
		})
	}
	if !balances.Active.IsZero() {
		balancesList = append(balancesList, &StakedBalance{
			State:     Active,
			Amount:    balances.Active,
			Validator: validator,
		})
	}
	if !balances.Deactivating.IsZero() {
		balancesList = append(balancesList, &StakedBalance{
			State:     Deactivating,
			Amount:    balances.Deactivating,
			Validator: validator,
		})
	}
	if !balances.Inactive.IsZero() {
		balancesList = append(balancesList, &StakedBalance{
			State:     Inactive,
			Amount:    balances.Inactive,
			Validator: validator,
		})
	}
	return balancesList
}
