package substrate

// These aren't used (besides bad rpc url) because the error strings don't provide more than "invalid transaction"

import (
	"strings"

	"github.com/cordialsys/crosschain/client/errors"
)

func CheckError(err error) errors.Status {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "insufficient funds for gas * price + value") {
		return errors.NoBalanceForGas
	}
	if strings.Contains(msg, "insufficient funds for transfer") {
		return errors.NoBalance
	}
	if strings.Contains(msg, "transaction underpriced") ||
		strings.Contains(msg, "response body closed") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "bad rpc url") {
		return errors.NetworkError
	}
	if strings.Contains(msg, "transaction already in block chain") ||
		strings.Contains(msg, "already known") {
		return errors.TransactionExists
	}
	return errors.UnknownError
}
