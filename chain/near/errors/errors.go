package errors

import (
	"errors"
	"fmt"
	"strings"

	clienterrors "github.com/cordialsys/crosschain/client/errors"
)

const (
	InvalidTransaction = "INVALID_TRANSACTION"
	ShardCongested     = "ShardCongested"
	ShardStuck         = "ShardStuck"
	ParseError         = "PARSE_ERROR"
	TimeoutError       = "TIMEOUT_ERROR"
	InternalError      = "INTERNAL_ERROR"
)

var ErrMissingSenderId = errors.New("near requires sender id, use 'sender-hash' format or pass --sender explicitly")
var ErrMissingPublicKey = errors.New("public key is required for NEAR transactions")
var ErrInvalidPublicKeyLength = errors.New("invalid public key lenght")

func CheckError(err error) clienterrors.Status {
	serr := err.Error()
	// We can retry if 'ShardCongested' and 'ShardStuck'
	// Invalid transaction otherwise
	if strings.Contains(serr, InvalidTransaction) && strings.Contains(serr, ShardCongested) {
		return clienterrors.FailedPrecondition
	} else if strings.Contains(serr, InvalidTransaction) && strings.Contains(serr, ShardStuck) {
		return clienterrors.FailedPrecondition
	} else if strings.Contains(serr, InvalidTransaction) {
		return clienterrors.TransactionFailure
	}

	if strings.Contains(serr, ParseError) {
		return clienterrors.TransactionFailure
	}

	if strings.Contains(serr, TimeoutError) {
		return clienterrors.TransactionTimedOut
	}

	if strings.Contains(serr, InternalError) {
		return clienterrors.NetworkError
	}
	return ""
}

func ErrInvalidPublicKeyLengthf(expected int, got int) error {
	return fmt.Errorf("%w: expected %d got %d", ErrInvalidPublicKeyLength, expected, got)
}
