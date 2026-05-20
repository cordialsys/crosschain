package canton

import (
	"strings"

	"github.com/cordialsys/crosschain/client/errors"
)

func CheckError(err error) errors.Status {
	if err == nil {
		return errors.UnknownError
	}
	msg := err.Error()

	if strings.Contains(msg, "401") || strings.Contains(msg, "403") ||
		strings.Contains(msg, "Unauthorized") || strings.Contains(msg, "unauthorized") {
		return errors.NetworkError
	}
	if strings.Contains(msg, "404") || strings.Contains(msg, "not found") ||
		strings.Contains(msg, "NOT_FOUND") {
		return errors.TransactionNotFound
	}
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
		return errors.NetworkError
	}
	if strings.Contains(msg, "ALREADY_EXISTS") || strings.Contains(msg, "already exists") {
		return errors.TransactionExists
	}
	if strings.Contains(msg, "FAILED_PRECONDITION") || strings.Contains(msg, "insufficient") {
		return errors.FailedPrecondition
	}
	if strings.Contains(msg, "EOF") || strings.Contains(msg, "connection") {
		return errors.NetworkError
	}

	return errors.NetworkError
}
