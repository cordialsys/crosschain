package errors

import (
	"fmt"
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

// The transaction could not be found
const TransactionNotFound Status = "TransactionNotFound"

// deadline exceeded and transaction can no longer be accepted
const TransactionTimedOut Status = "TransactionTimedOut"

// A network error occured -- there may be nothing wrong with the transaction
const NetworkError Status = "NetworkError"

// No outcome for this error known
const UnknownError Status = "UnknownError"

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

func TransactionExistsf(format string, args ...interface{}) error {
	return &Error{
		Status:  TransactionExists,
		Message: fmt.Sprintf(format, args...),
	}
}

func TransactionNotFoundf(format string, args ...interface{}) error {
	return &Error{
		Status:  TransactionNotFound,
		Message: fmt.Sprintf(format, args...),
	}
}

func Unknownf(format string, args ...interface{}) error {
	return &Error{
		Status:  UnknownError,
		Message: fmt.Sprintf(format, args...),
	}
}
