package newchain

import (
	"strings"

	"github.com/cordialsys/crosschain/chain/cardano/client/types"
	"github.com/cordialsys/crosschain/client/errors"
)

func CheckError(err error) errors.Status {
	if strings.Contains(err.Error(), types.CodeMempoolFull) {
		return errors.NetworkError
	}
	if strings.Contains(err.Error(), types.CodeDailyRequestLimitExceeded) {
		return errors.NetworkError
	}
	if strings.Contains(err.Error(), types.CodeInternalServerError) {
		return errors.NetworkError
	}
	if strings.Contains(err.Error(), types.CodeNotAuthenticated) {
		return errors.NetworkError
	}
	if strings.Contains(err.Error(), types.CodeRateLimitExceeded) {
		return errors.NetworkError
	}
	if strings.Contains(err.Error(), types.CodeUserAutoBannedForFlooding) {
		return errors.NetworkError
	}
	return errors.UnknownError
}
