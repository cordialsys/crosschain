package sui

import (
	"strings"

	xc "github.com/jumpcrypto/crosschain"
)

func CheckError(err error) xc.ClientError {
	msg := strings.ToLower(err.Error())
	// if strings.Contains(msg, "insufficient funds for gas * price + value") {
	// 	return xc.NoBalanceForGas
	// }
	// if strings.Contains(msg, "insufficient funds for transfer") {
	// 	return xc.NoBalance
	// }
	// if strings.Contains(msg, "transaction underpriced") ||
	// 	strings.Contains(msg, "response body closed") ||
	// 	strings.Contains(msg, "not found") ||
	// 	strings.Contains(msg, "eof") ||
	// 	strings.Contains(msg, "bad rpc url") {
	// 	return xc.NetworkError
	// }
	// if strings.Contains(msg, "transaction already in block chain") ||
	// 	strings.Contains(msg, "already known") {
	// 	return xc.TransactionExists
	// }
	if strings.Contains(msg, "eof") {
		return xc.NetworkError
	}

	return xc.UnknownError
}
