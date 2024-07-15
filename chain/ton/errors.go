package ton

import (
	"strings"

	xclient "github.com/cordialsys/crosschain/client"
)

func CheckError(err error) xclient.ClientError {
	if strings.Contains(err.Error(), "duplicate message") {
		return xclient.TransactionExists
	}
	return ""
}
