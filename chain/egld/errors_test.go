package egld_test

import (
	"errors"
	"testing"

	"github.com/cordialsys/crosschain/chain/egld"
	"github.com/cordialsys/crosschain/chain/egld/types"
	xcerrors "github.com/cordialsys/crosschain/client/errors"
	"github.com/stretchr/testify/require"
)

func TestCheckError_ApiError(t *testing.T) {
	tests := []struct {
		name     string
		apiError *types.ApiError
		want     xcerrors.Status
	}{
		{
			name: "Insufficient funds",
			apiError: &types.ApiError{
				Message: "insufficient funds",
				Code:    "insufficientFunds",
			},
			want: xcerrors.NoBalance,
		},
		{
			name: "Insufficient gas",
			apiError: &types.ApiError{
				Message: "gas limit too low",
				Code:    "insufficientGasLimit",
			},
			want: xcerrors.NoBalanceForGas,
		},
		{
			name: "Transaction not found",
			apiError: &types.ApiError{
				Message: "transaction not found",
				Code:    "notFound",
			},
			want: xcerrors.TransactionNotFound,
		},
		{
			name: "Transaction failed",
			apiError: &types.ApiError{
				Message: "execution failed",
				Code:    "transactionFailed",
			},
			want: xcerrors.TransactionFailure,
		},
		{
			name: "Duplicate transaction",
			apiError: &types.ApiError{
				Message: "transaction already exists",
				Code:    "duplicateTransaction",
			},
			want: xcerrors.TransactionExists,
		},
		{
			name: "Rate limit",
			apiError: &types.ApiError{
				Message: "too many requests",
				Code:    "rateLimit",
			},
			want: xcerrors.NetworkError,
		},
		{
			name: "Internal issue",
			apiError: &types.ApiError{
				Message: "checksum failed",
				Code:    "internal_issue",
			},
			want: xcerrors.NetworkError,
		},
		{
			name: "Timeout",
			apiError: &types.ApiError{
				Message: "request timed out",
				Code:    "timeout",
			},
			want: xcerrors.TransactionTimedOut,
		},
		{
			name: "Unknown error code with message fallback",
			apiError: &types.ApiError{
				Message: "insufficient balance for transfer",
				Code:    "unknownCode",
			},
			want: xcerrors.NoBalance,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := egld.CheckError(tt.apiError)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCheckError_StringError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want xcerrors.Status
	}{
		{
			name: "Transaction not found",
			err:  errors.New("transaction not found"),
			want: xcerrors.TransactionNotFound,
		},
		{
			name: "Account not found",
			err:  errors.New("account not found"),
			want: xcerrors.TransactionNotFound,
		},
		{
			name: "Insufficient funds",
			err:  errors.New("insufficient funds for transfer"),
			want: xcerrors.NoBalance,
		},
		{
			name: "Insufficient balance",
			err:  errors.New("insufficient balance"),
			want: xcerrors.NoBalance,
		},
		{
			name: "Insufficient gas",
			err:  errors.New("insufficient gas limit"),
			want: xcerrors.NoBalanceForGas,
		},
		{
			name: "Connection refused",
			err:  errors.New("connection refused"),
			want: xcerrors.NetworkError,
		},
		{
			name: "Timeout",
			err:  errors.New("request timed out"),
			want: xcerrors.NetworkError,
		},
		{
			name: "EOF",
			err:  errors.New("EOF"),
			want: xcerrors.NetworkError,
		},
		{
			name: "Rate limit",
			err:  errors.New("too many requests, please retry after 60s"),
			want: xcerrors.NetworkError,
		},
		{
			name: "Transaction already exists",
			err:  errors.New("transaction already exists in mempool"),
			want: xcerrors.TransactionExists,
		},
		{
			name: "Duplicate",
			err:  errors.New("duplicate transaction"),
			want: xcerrors.TransactionExists,
		},
		{
			name: "Invalid address",
			err:  errors.New("invalid EGLD address format"),
			want: xcerrors.TransactionFailure,
		},
		{
			name: "Failed to decode bech32",
			err:  errors.New("failed to decode bech32 address"),
			want: xcerrors.TransactionFailure,
		},
		{
			name: "Transaction failed",
			err:  errors.New("transaction execution failed"),
			want: xcerrors.TransactionFailure,
		},
		{
			name: "Unknown error",
			err:  errors.New("some unknown error"),
			want: xcerrors.UnknownError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := egld.CheckError(tt.err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCheckError_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want xcerrors.Status
	}{
		{
			name: "Mixed case - insufficient funds",
			err:  errors.New("Insufficient Funds For Transfer"),
			want: xcerrors.NoBalance,
		},
		{
			name: "Uppercase - transaction not found",
			err:  errors.New("TRANSACTION NOT FOUND"),
			want: xcerrors.TransactionNotFound,
		},
		{
			name: "Mixed case - connection refused",
			err:  errors.New("Connection Refused"),
			want: xcerrors.NetworkError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := egld.CheckError(tt.err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCheckError_Nil(t *testing.T) {
	got := egld.CheckError(nil)
	require.Equal(t, xcerrors.Status(""), got)
}

func TestCheckError_EmptyApiError(t *testing.T) {
	apiErr := &types.ApiError{}
	got := egld.CheckError(apiErr)
	require.Equal(t, xcerrors.Status(""), got)
}
