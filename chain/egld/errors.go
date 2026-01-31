package egld

import (
	"strings"

	"github.com/cordialsys/crosschain/chain/egld/types"
	"github.com/cordialsys/crosschain/client/errors"
)

func CheckError(err error) errors.Status {
	if err == nil {
		return ""
	}

	// Check if it's an ApiError
	if apiErr, ok := err.(*types.ApiError); ok {
		return checkApiError(apiErr)
	}

	// Fallback to string matching
	msg := strings.ToLower(err.Error())
	return checkErrorMessage(msg)
}

func checkApiError(apiErr *types.ApiError) errors.Status {
	if apiErr == nil || !apiErr.HasError() {
		return ""
	}

	code := strings.ToLower(apiErr.Code)
	msg := strings.ToLower(apiErr.Message)

	// Check error codes first (more specific)
	switch code {
	case "insufficientfunds", "insufficient_funds":
		return errors.NoBalance
	case "insufficientgaslimit", "insufficient_gas_limit":
		return errors.NoBalanceForGas
	case "transactionnotfound", "transaction_not_found", "notfound", "not_found":
		return errors.TransactionNotFound
	case "transactionfailed", "transaction_failed":
		return errors.TransactionFailure
	case "duplicatetransaction", "duplicate_transaction":
		return errors.TransactionExists
	case "timeout", "requesttimeout", "request_timeout":
		return errors.TransactionTimedOut
	case "ratelimit", "rate_limit", "toomanyrequests", "too_many_requests":
		return errors.NetworkError
	case "internal_issue", "internalerror", "internal_error":
		return errors.NetworkError
	}

	// Fall back to message matching
	return checkErrorMessage(msg)
}

func checkErrorMessage(msg string) errors.Status {
	// Transaction not found errors
	if strings.Contains(msg, "transaction not found") ||
		strings.Contains(msg, "tx not found") {
		return errors.TransactionNotFound
	}

	// Account/address not found
	if strings.Contains(msg, "account not found") ||
		strings.Contains(msg, "address not found") {
		return errors.TransactionNotFound
	}

	// Balance errors
	if strings.Contains(msg, "insufficient funds") ||
		strings.Contains(msg, "insufficient balance") ||
		strings.Contains(msg, "not enough balance") {
		return errors.NoBalance
	}

	// Gas errors
	if strings.Contains(msg, "insufficient gas") ||
		strings.Contains(msg, "not enough gas") ||
		strings.Contains(msg, "gas limit") {
		return errors.NoBalanceForGas
	}

	// Network errors
	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "timed out") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "network is unreachable") {
		return errors.NetworkError
	}

	// Rate limiting
	if strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "retry after") {
		return errors.NetworkError
	}

	// Transaction already exists
	if strings.Contains(msg, "transaction already exists") ||
		strings.Contains(msg, "already known") ||
		strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "already in mempool") {
		return errors.TransactionExists
	}

	// Invalid address/transaction
	if strings.Contains(msg, "invalid egld address") ||
		strings.Contains(msg, "invalid address") ||
		strings.Contains(msg, "failed to decode bech32") ||
		strings.Contains(msg, "invalid transaction") ||
		strings.Contains(msg, "invalid signature") {
		return errors.TransactionFailure
	}

	// Transaction execution failure
	if strings.Contains(msg, "transaction failed") ||
		strings.Contains(msg, "execution failed") ||
		strings.Contains(msg, "transaction reverted") {
		return errors.TransactionFailure
	}

	return errors.UnknownError
}
