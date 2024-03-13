package sui

import (
	"strings"

	xclient "github.com/cordialsys/crosschain/client"
)

func CheckError(err error) xclient.ClientError {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "eof") {
		return xclient.NetworkError
	}
	if strings.Contains(msg, "transaction execution failed") {
		return xclient.TransactionFailure
	}

	return xclient.UnknownError
}
