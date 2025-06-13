package evm

import (
	"strings"

	"github.com/cordialsys/crosschain/client/errors"
)

func CheckError(err error) errors.Status {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "insufficient funds for gas * price + value") {
		return errors.NoBalanceForGas
	}
	if strings.Contains(msg, "insufficient funds for transfer") ||
		strings.Contains(msg, "insufficient funds of the sender") {
		return errors.NoBalance
	}
	// Polygon seems to return "transaction underpriced" but still forwarding the tx the chain,
	// that eventually accepts the tx.
	// By returning a network error, we expect the tx to be retried and eventually found on chain
	// as "already known".
	if strings.Contains(msg, "transaction underpriced") ||
		strings.Contains(msg, "response body closed") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "eof") {
		return errors.NetworkError
	}
	if strings.Contains(msg, "transaction already in block chain") ||
		strings.Contains(msg, "already known") ||
		strings.Contains(msg, "known transaction:") ||
		strings.Contains(msg, "transaction already imported") {
		return errors.TransactionExists
	}
	return errors.UnknownError
}
