package monero

import (
	"strings"

	"github.com/cordialsys/crosschain/client/errors"
)

func CheckError(err error) errors.Status {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not found") || strings.Contains(msg, "missed_tx") {
		return errors.TransactionNotFound
	}
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
		return errors.NetworkError
	}
	return ""
}
