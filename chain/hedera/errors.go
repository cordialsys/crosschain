package hedera

import (
	"strings"

	"github.com/cordialsys/crosschain/client/errors"
)

func CheckError(err error) errors.Status {
	serr := err.Error()
	if strings.Contains(serr, "DUPLICATE_TRANSACTION") {
		return errors.TransactionExists
	}
	if strings.Contains(serr, "INSUFFICIENT_PAYER_BALANCE") || strings.Contains(serr, "INSUFFICIENT_ACCOUNT_BALANCE") {
		return errors.NoBalance
	}
	if strings.Contains(serr, "TRANSACTION_EXPIRED") {
		return errors.TransactionTimedOut
	}

	return ""
}
