package sui

import (
	"strings"

	xc "github.com/cordialsys/crosschain"
)

func CheckError(err error) xc.ClientError {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "eof") {
		return xc.NetworkError
	}
	if strings.Contains(msg, "transaction execution failed") {
		return xc.TransactionFailure
	}

	return xc.UnknownError
}
