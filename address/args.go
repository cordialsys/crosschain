package address

import (
	xc "github.com/cordialsys/crosschain"
)

type addressOptions struct {
	algorithm *xc.SignatureType
	format    *xc.AddressFormat
	viewKey   *string
}

type AddressOptions interface {
	GetAlgorithmType() (xc.SignatureType, bool)
	GetFormat() (xc.AddressFormat, bool)
	GetViewKey() (string, bool)
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

func (opts *addressOptions) GetFormat() (xc.AddressFormat, bool) {
	return get(opts.format)
}

func (opts *addressOptions) GetViewKey() (string, bool) {
	return get(opts.viewKey)
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

func OptionFormat(format xc.AddressFormat) AddressOption {
	return func(opts *addressOptions) error {
		if string(format) != "" {
			opts.format = &format
			return nil
		}

		return nil
	}
}

// OptionViewKey provides a (private) view key for privacy chains like Monero.
// Required for chains that use view keys to scan for owned outputs.
func OptionViewKey(viewKey string) AddressOption {
	return func(opts *addressOptions) error {
		if viewKey != "" {
			opts.viewKey = &viewKey
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
