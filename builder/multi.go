package builder

import (
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

type Sender struct {
	address        xc.Address
	publicKey      []byte
	options        builderOptions
	appliedOptions []BuilderOption
}

type Receiver struct {
	address        xc.Address
	amount         xc.AmountBlockchain
	options        builderOptions
	appliedOptions []BuilderOption
}

func NewSender(address xc.Address, publicKey []byte, options ...BuilderOption) (*Sender, error) {
	builderOptions := newBuilderOptions()
	for _, opt := range options {
		err := opt(&builderOptions)
		if err != nil {
			return nil, err
		}
	}
	return &Sender{
		address,
		publicKey,
		builderOptions,
		options,
	}, nil
}

func NewReceiver(address xc.Address, amount xc.AmountBlockchain, options ...BuilderOption) (*Receiver, error) {
	builderOptions := newBuilderOptions()
	for _, opt := range options {
		err := opt(&builderOptions)
		if err != nil {
			return nil, err
		}
	}
	return &Receiver{
		address,
		amount,
		builderOptions,
		options,
	}, nil
}

type MultiTransferArgs struct {
	spenders       []*Sender
	receivers      []*Receiver
	options        builderOptions
	appliedOptions []BuilderOption
}

func NewMultiTransferArgs(spenders []*Sender, receivers []*Receiver, options ...BuilderOption) (*MultiTransferArgs, error) {
	builderOptions := newBuilderOptions()
	for _, opt := range options {
		err := opt(&builderOptions)
		if err != nil {
			return nil, err
		}
	}
	return &MultiTransferArgs{
		spenders,
		receivers,
		builderOptions,
		options,
	}, nil
}

func (args *Sender) GetFrom() xc.Address { return args.address }

func (args *Sender) GetPublicKey() []byte {
	return args.publicKey
}

func (args *Receiver) GetTo() xc.Address              { return args.address }
func (args *Receiver) GetAmount() xc.AmountBlockchain { return args.amount }

func (args *Receiver) GetContract() (xc.ContractAddress, bool) {
	return args.options.GetContract()
}

func (args *Receiver) GetDecimals() (int, bool) {
	return args.options.GetDecimals()
}

func (args *Receiver) GetMemo() (string, bool) {
	return args.options.GetMemo()
}

func (args *MultiTransferArgs) Spenders() []*Sender {
	return args.spenders
}

func (args *MultiTransferArgs) Receivers() []*Receiver {
	return args.receivers
}

func (args *MultiTransferArgs) GetPriority() (xc.GasFeePriority, bool) {
	return args.options.GetPriority()
}
func (args *MultiTransferArgs) GetFeePayer() (xc.Address, bool) {
	return args.options.GetFeePayer()
}

func (args *MultiTransferArgs) GetFeePayerPublicKey() ([]byte, bool) {
	return args.options.GetFeePayerPublicKey()
}

func (args *MultiTransferArgs) GetMemo() (string, bool) {
	return args.options.GetMemo()
}

func (args *MultiTransferArgs) AsTransfers() ([]*TransferArgs, error) {
	transfers := make([]*TransferArgs, len(args.spenders))
	if len(args.spenders) != len(args.receivers) {
		return nil, errors.New("spenders and receivers must be the same length")
	}
	for i := range args.spenders {
		spender := args.spenders[i]
		receiver := args.receivers[i]
		allOptions := []BuilderOption{}
		allOptions = append(allOptions, spender.appliedOptions...)
		allOptions = append(allOptions, receiver.appliedOptions...)
		// apply args options last so they take precedence
		allOptions = append(allOptions, args.appliedOptions...)

		transferArgs, err := NewTransferArgs(spender.address, receiver.address, receiver.amount, allOptions...)
		if err != nil {
			return nil, err
		}
		transfers[i] = &transferArgs
	}
	return transfers, nil
}

// Deduct fee from the first matching receiver
// Used for inclusive fee spending.
func (args *MultiTransferArgs) DeductFee(amount xc.AmountBlockchain, chainId xc.NativeAsset, contract xc.ContractAddress) error {
	// funge empty contract with the chainId
	if contract == "" {
		contract = xc.ContractAddress(chainId)
	}
	for _, receiver := range args.receivers {
		receiverContract, _ := receiver.GetContract()
		if receiverContract == "" {
			receiverContract = xc.ContractAddress(chainId)
		}

		if receiverContract == contract {
			if receiver.amount.Int().Cmp(amount.Int()) >= 0 {
				receiver.amount = receiver.amount.Sub(&amount)
				return nil
			}
		}
	}
	return fmt.Errorf("no matching receiver found to deduct fee of %s %s", amount.String(), contract)
}
