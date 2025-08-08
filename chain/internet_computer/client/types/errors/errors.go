package errors

import "fmt"

const TransactionTooOldError = "transaction too old"
const BadFeeError = "bad fee"
const TransactionDuplicateError = "transaction duplicate"
const CreatedInFutureError = "transaction created in future"
const InsufficientFundsError = "insufficient funds"
const UnknownError = "unknown error"
const BadBurnError = "bad burn"
const GenericError = "generic error"

func TransactionTooOld() string {
	return TransactionTooOldError
}

func BadFee(expected uint64) string {
	return fmt.Sprintf("%s, expected: %d", BadFeeError, expected)
}

func TransactionDuplicate(index uint64) string {
	return fmt.Sprintf("%s, duplication index: %d", TransactionDuplicateError, index)
}

func CreatedInFuture() string {
	return CreatedInFutureError
}

func InsufficientFunds(balance uint64) string {
	return fmt.Sprintf("%s, balance: %d", InsufficientFundsError, balance)
}

func Unknown() string {
	return UnknownError
}

func BadBurn(minAmount uint64) string {
	return fmt.Sprintf("%s, min burn amount: %d", BadBurnError, minAmount)
}

func Generic(code uint64, message string) string {
	return fmt.Sprintf("generic error, code: %d, message: %s", code, message)
}
