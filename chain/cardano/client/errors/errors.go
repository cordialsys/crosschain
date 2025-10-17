package errors

import "fmt"

func AddressInfof(err error) error {
	return fmt.Errorf("failed to fetch address info: %w", err)
}

func FeeEstimationf(err error) error {
	return fmt.Errorf("failed to create transaction for fee estimation: %w", err)
}

func BaseInputf(err error) error {
	return fmt.Errorf("failed to fetch base transaction input: %w", err)
}

func ProtocolParamsf(err error) error {
	return fmt.Errorf("failed to fetch protocol parameters: %w", err)
}

func CalculateTxFee(err error) error {
	return fmt.Errorf("failed to calculate transaction fee: %w", err)
}

func DepositValuef(err error) error {
	return fmt.Errorf("failed to parse key deposit value: %w", err)
}
