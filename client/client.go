package client

import (
	"context"

	xc "github.com/cordialsys/crosschain"
)

// Client is a client that can fetch data and submit tx to a public blockchain
type Client interface {
	FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error)

	SubmitTx(ctx context.Context, tx xc.Tx) error

	// Fetching transaction info - legacy endpoint
	FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error)
	// Fetching transaction info
	FetchTxInfo(ctx context.Context, txHash xc.TxHash) (TxInfo, error)

	FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error)
	FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error)
}

type EstimateGasFunc func(native xc.NativeAsset) (xc.AmountBlockchain, error)

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
const TransactionTimedout ClientError = "TransactionTimedOut"

// A network error occured -- there may be nothing wrong with the transaction
const NetworkError ClientError = "NetworkError"

// No outcome for this error known
const UnknownError ClientError = "UnknownError"
