package bitcoin

import (
	"strings"

	"github.com/cordialsys/crosschain/client/errors"
)

func CheckError(err error) errors.Status {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "txn-mempool-conflict") ||
		strings.Contains(msg, "bad-txns-inputs-missingorspent") {
		return errors.TransactionFailure
	}
	if strings.Contains(msg, "response body closed") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "could not find a result on blockchair") ||
		strings.Contains(msg, "eof") {
		return errors.NetworkError
	}
	if strings.Contains(msg, "transaction already in block chain") ||
		strings.Contains(msg, "already known") {
		return errors.TransactionExists
	}
	return errors.UnknownError
}
