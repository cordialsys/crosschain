package tron

import (
	"errors"
	"strings"

	clienterrors "github.com/cordialsys/crosschain/client/errors"
)

var ErrFailedToFetchBaseInput = errors.New("failed to fetch base input")
var ErrRequiresResubmission = errors.New("ResubmissionRequired")

func CheckError(err error) clienterrors.Status {
	e := err.Error()
	// Map to NetworkError so resubmission is guaranteed on the treasury side
	// TODO: Add proper error handling on the Treasury side, and remove this mapping
	if strings.Contains(e, "ResubmissionRequired") {
		return clienterrors.NetworkError
	}
	if strings.Contains(e, "SIGERROR") {
		return clienterrors.TransactionFailure
	}
	// There is no specific error for insufficient gas funds
	// balance is not sufficient is a subset of "CONTRACT_VALIDATE_ERROR"
	// and should be checked for before it
	if strings.Contains(e, "balance is not sufficient") {
		return clienterrors.NoBalance
	}
	if strings.Contains(e, "CONTRACT_VALIDATE_ERROR") {
		return clienterrors.TransactionFailure
	}
	if strings.Contains(e, "InvalidProtocolBufferException") {
		return clienterrors.TransactionFailure
	}
	if strings.Contains(e, "TAPOS_ERROR") {
		return clienterrors.TransactionTimedOut
	}
	return ""
}
