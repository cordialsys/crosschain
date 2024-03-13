package evm

import (
	"strings"

	xclient "github.com/cordialsys/crosschain/client"
)

func CheckError(err error) xclient.ClientError {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "insufficient funds for gas * price + value") {
		return xclient.NoBalanceForGas
	}
	if strings.Contains(msg, "insufficient funds for transfer") ||
		strings.Contains(msg, "insufficient funds of the sender") {
		return xclient.NoBalance
	}
	// Polygon seems to return "transaction underpriced" but still forwarding the tx the chain,
	// that eventually accepts the tx.
	// By returning a network error, we expect the tx to be retried and eventually found on chain
	// as "already known".
	if strings.Contains(msg, "transaction underpriced") ||
		strings.Contains(msg, "response body closed") ||
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
