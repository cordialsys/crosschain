package xrp

import (
	"strings"

	"github.com/cordialsys/crosschain/client/errors"
)

func CheckError(err error) errors.Status {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "eof") {
		return errors.NetworkError
	}
	return ""
}
