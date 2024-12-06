package sui

import (
	"strings"

	"github.com/cordialsys/crosschain/client/errors"
)

func CheckError(err error) errors.Status {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "eof") {
		return errors.NetworkError
	}
	if strings.Contains(msg, "transaction execution failed") {
		return errors.TransactionFailure
	}

	return errors.UnknownError
}
