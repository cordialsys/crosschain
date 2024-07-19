package client

import (
	"context"

	xc "github.com/cordialsys/crosschain"
)

// Client is a client that can fetch data and submit tx to a public blockchain
type Client interface {
	// Fetch the basic transaction input for any new transaction
	FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error)

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

type StakingClient interface {
	// Provider inputs for staking transactions
	SetStakingInput(stakingInput xc.StakingInput)
}

type FullClient interface {
	Client
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
