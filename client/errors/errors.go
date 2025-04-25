package errors

import (
	"fmt"

	"google.golang.org/grpc/codes"
)

type Status string

// A transaction terminally failed due to no balance
const NoBalance Status = "NoBalance"

// A transaction terminally failed due to no balance after accounting for gas cost
const NoBalanceForGas Status = "NoBalanceForGas"

// A transaction terminally failed due to another reason
const TransactionFailure Status = "TransactionFailure"

// A transaction failed to submit because it already exists
const TransactionExists Status = "TransactionExists"

// The transaction could not be found on chain
const TransactionNotFound Status = "TransactionNotFound"

// deadline exceeded and transaction can no longer be accepted
const TransactionTimedOut Status = "TransactionTimedOut"

// A network error occured -- there may be nothing wrong with the transaction
const NetworkError Status = "NetworkError"

// No outcome for this error known
const UnknownError Status = "UnknownError"

// Failed to due to an on-chain condition that could resolve in time.
const FailedPrecondition Status = "FailedPrecondition"

func (s Status) ToGrpcCode() (codes.Code, bool) {
	switch s {
	case TransactionNotFound:
		// transaction not found
		return codes.NotFound, true
	case FailedPrecondition:
		// on-chain error, retryable
		return codes.FailedPrecondition, true
	case TransactionExists:
		// transaction already exists, should fetch
		return codes.AlreadyExists, true
	case NetworkError:
		// network error, retryable
		return codes.Unavailable, true
	case NoBalance, NoBalanceForGas:
		// no balance
		return codes.OutOfRange, true
	case TransactionTimedOut, TransactionFailure:
		// transaction will _not_ work
		return codes.Aborted, true
	}
	return codes.Unknown, false
}

func FromGrpcCode(code codes.Code) (Status, bool) {
	switch code {
	case codes.NotFound:
		return TransactionNotFound, true
	case codes.FailedPrecondition:
		return FailedPrecondition, true
	case codes.AlreadyExists:
		return TransactionExists, true
	case codes.Unavailable:
		return NetworkError, true
	case codes.OutOfRange:
		return NoBalance, true
	case codes.Aborted:
		return TransactionFailure, true
	}
	return UnknownError, false
}

type Error struct {
	Status  Status
	Message string
}

var _ error = &Error{}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Status, e.Message)
}

func Errorf(status Status, format string, args ...interface{}) error {
	return &Error{
		Status:  status,
		Message: fmt.Sprintf(format, args...),
	}
}

// Used to indicate that the transaction already exists on chain,
// when attempting to submit.
func TransactionExistsf(format string, args ...interface{}) error {
	return &Error{
		Status:  TransactionExists,
		Message: fmt.Sprintf(format, args...),
	}
}

// Used when a transaction is not found on chain.
func TransactionNotFoundf(format string, args ...interface{}) error {
	return &Error{
		Status:  TransactionNotFound,
		Message: fmt.Sprintf(format, args...),
	}
}

// Used when a transaction is not found on chain.
func FailedPreconditionf(format string, args ...interface{}) error {
	return &Error{
		Status:  FailedPrecondition,
		Message: fmt.Sprintf(format, args...),
	}
}

func Unknownf(format string, args ...interface{}) error {
	return &Error{
		Status:  UnknownError,
		Message: fmt.Sprintf(format, args...),
	}
}
