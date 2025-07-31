package address

import (
	xc "github.com/cordialsys/crosschain"
)

type addressOptions struct {
	algorithm *xc.SignatureType
	contract  *xc.ContractAddress
}

type AddressOptions interface {
	GetAlgorithmType() (xc.SignatureType, bool)
	GetContract() (xc.ContractAddress, bool)
}

var _ AddressOptions = &addressOptions{}

func get[T any](arg *T) (T, bool) {
	if arg == nil {
		var zero T
		return zero, false
	}
	return *arg, true
}

func (opts *addressOptions) GetAlgorithmType() (xc.SignatureType, bool) {
	return get(opts.algorithm)
}

func (opts *addressOptions) GetContract() (xc.ContractAddress, bool) {
	return get(opts.contract)
}

type AddressOption func(opts *addressOptions) error

func OptionAlgorithm(algorithm xc.SignatureType) AddressOption {
	return func(opts *addressOptions) error {
		if algorithm != "" {
			opts.algorithm = &algorithm
			return nil
		}

		return nil
	}
}

func OptionContract(contract xc.ContractAddress) AddressOption {
	return func(opts *addressOptions) error {
		if contract != "" {
			opts.contract = &contract
			return nil
		}

		return nil
	}
}

func NewAddressOptions(opts ...AddressOption) (addressOptions, error) {
	addressOptions := addressOptions{}
	for _, opt := range opts {
		err := opt(&addressOptions)
		if err != nil {
			return addressOptions, err
		}
	}

	return addressOptions, nil
}
