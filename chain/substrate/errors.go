package substrate

// These aren't used (besides bad rpc url) because the error strings don't provide more than "invalid transaction"

import (
	"strings"

	xclient "github.com/cordialsys/crosschain/client"
)

func CheckError(err error) xclient.ClientError {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "insufficient funds for gas * price + value") {
		return xclient.NoBalanceForGas
	}
	if strings.Contains(msg, "insufficient funds for transfer") {
		return xclient.NoBalance
	}
	if strings.Contains(msg, "transaction underpriced") ||
		strings.Contains(msg, "response body closed") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "bad rpc url") {
		return xclient.NetworkError
	}
	if strings.Contains(msg, "transaction already in block chain") ||
		strings.Contains(msg, "already known") {
		return xclient.TransactionExists
	}
	return xclient.UnknownError
}
