package bitcoin

import (
	"strings"

	xclient "github.com/cordialsys/crosschain/client"
)

func CheckError(err error) xclient.ClientError {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "txn-mempool-conflict") ||
		strings.Contains(msg, "bad-txns-inputs-missingorspent") {
		return xclient.TransactionFailure
	}
	if strings.Contains(msg, "response body closed") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "could not find a result on blockchair") ||
		strings.Contains(msg, "eof") {
		return xclient.NetworkError
	}
	if strings.Contains(msg, "transaction already in block chain") ||
		strings.Contains(msg, "already known") {
		return xclient.TransactionExists
	}
	return xclient.UnknownError
}
