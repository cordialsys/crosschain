package solana

import (
	"strings"

	"github.com/cordialsys/crosschain/client/errors"
)

func CheckError(err error) errors.Status {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "transaction underpriced") {
		return errors.TransactionFailure
	}
	if strings.Contains(msg, "insufficient funds for gas * price + value") ||
		strings.Contains(msg, "insufficient funds for rent") {
		return errors.NoBalanceForGas
	}
	if strings.Contains(msg, "insufficient funds for transfer") {
		return errors.NoBalance
	}
	if strings.Contains(msg, "blockhash not found") {
		return errors.TransactionTimedOut
	}
	if strings.Contains(msg, "transaction already in block chain") ||
		strings.Contains(msg, "transaction has already been processed") {
		return errors.TransactionExists
	}
	if strings.Contains(msg, "response body closed") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "eof") {
		return errors.NetworkError
	}

	return errors.UnknownError
}
