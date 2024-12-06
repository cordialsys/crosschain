package ton

import (
	"strings"

	"github.com/cordialsys/crosschain/client/errors"
)

func CheckError(err error) errors.Status {
	if strings.Contains(err.Error(), "duplicate message") {
		return errors.TransactionExists
	}
	return ""
}
