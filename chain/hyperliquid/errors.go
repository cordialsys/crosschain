package newchain

import (
	"strings"

	"github.com/cordialsys/crosschain/client/errors"
)

func CheckError(err error) errors.Status {
	errMsg := err.Error()
	if strings.Contains(errMsg, "Insufficient balance for token transfer") {
		return errors.NoBalance
	}

	// In 99% cases this is related to invalid signature
	if strings.Contains(errMsg, "Must deposit before performing actions. User") {
		return errors.TransactionFailure
	}

	if strings.Contains(errMsg, "Invalid nonce: duplicate nonce") {
		return errors.TransactionExists
	}

	// Other nonce related issue
	if strings.Contains(errMsg, "Invalid nonce: )") {
		return errors.TransactionFailure
	}

	if strings.Contains(errMsg, "TransactionNotFound") {
		return errors.TransactionNotFound
	}

	return ""
}
