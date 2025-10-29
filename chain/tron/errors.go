package tron

import (
	"errors"

	clienterrors "github.com/cordialsys/crosschain/client/errors"
)

var ErrFailedToFetchBaseInput = errors.New("failed to fetch base input")
var ErrRequiresResubmission = errors.New("ResubmissionRequired")

func CheckError(err error) clienterrors.Status {
	return ""
}
