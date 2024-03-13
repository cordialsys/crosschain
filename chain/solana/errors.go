package solana

import (
	"strings"

	xclient "github.com/cordialsys/crosschain/client"
)

func CheckError(err error) xclient.ClientError {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "transaction underpriced") {
		return xclient.TransactionFailure
	}
	if strings.Contains(msg, "insufficient funds for gas * price + value") ||
		strings.Contains(msg, "insufficient funds for rent") {
		return xclient.NoBalanceForGas
	}
	if strings.Contains(msg, "insufficient funds for transfer") {
		return xclient.NoBalance
	}
	if strings.Contains(msg, "response body closed") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "eof") {
		return xclient.NetworkError
	}
	if strings.Contains(msg, "transaction already in block chain") ||
		strings.Contains(msg, "already known") {
		return xclient.TransactionExists
	}
	return xclient.UnknownError
}
