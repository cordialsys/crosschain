package internet_computer

import (
	"strings"

	icperrors "github.com/cordialsys/crosschain/chain/internet_computer/client/types/errors"
	"github.com/cordialsys/crosschain/client/errors"
)

func CheckError(err error) errors.Status {
	errMsg := err.Error()
	if strings.Contains(errMsg, icperrors.CreatedInFutureError) {
		return errors.TransactionFailure
	}
	if strings.Contains(errMsg, icperrors.TransactionTooOldError) {
		return errors.TransactionFailure
	}
	if strings.Contains(errMsg, icperrors.BadFeeError) {
		return errors.TransactionFailure
	}

	if strings.Contains(errMsg, icperrors.InsufficientFundsError) {
		return errors.NoBalance
	}
	if strings.Contains(errMsg, icperrors.TransactionDuplicateError) {
		return errors.TransactionExists
	}

	if strings.Contains(errMsg, icperrors.UnknownError) {
		return errors.UnknownError
	}

	return ""
}
