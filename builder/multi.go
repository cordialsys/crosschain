package builder

import (
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

type Spender struct {
	address        xc.Address
	options        builderOptions
	appliedOptions []BuilderOption
}

type Receiver struct {
	address        xc.Address
	amount         xc.AmountBlockchain
	options        builderOptions
	appliedOptions []BuilderOption
}

func NewSpender(address xc.Address, options ...BuilderOption) (*Spender, error) {
	builderOptions := newBuilderOptions()
	for _, opt := range options {
		err := opt(&builderOptions)
		if err != nil {
			return nil, err
		}
	}
	return &Spender{
		address,
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
	spenders       []*Spender
	receivers      []*Receiver
	options        builderOptions
	appliedOptions []BuilderOption
}

func NewMultiTransferArgs(spenders []*Spender, receivers []*Receiver, options ...BuilderOption) (*MultiTransferArgs, error) {
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

func (args *Spender) GetFrom() xc.Address { return args.address }
func (args *Spender) GetContract() (xc.ContractAddress, bool) {
	return args.options.GetContract()
}
func (args *Spender) GetDecimals() (int, bool) {
	return args.options.GetDecimals()
}
func (args *Spender) GetPublicKey() ([]byte, bool) {
	return args.options.GetPublicKey()
}

func (args *Receiver) GetTo() xc.Address              { return args.address }
func (args *Receiver) GetAmount() xc.AmountBlockchain { return args.amount }

func (args *Receiver) GetContract() (xc.ContractAddress, bool) {
	return args.options.GetContract()
}

func (args *Receiver) GetDecimals() (int, bool) {
	return args.options.GetDecimals()
}

func (args *MultiTransferArgs) Spenders() []*Spender {
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

func (args *MultiTransferArgs) AsTransfers() ([]*TransferArgs, error) {
	transfers := make([]*TransferArgs, len(args.spenders))
	if len(args.spenders) != len(args.receivers) {
		return nil, errors.New("spenders and receivers must be the same length")
	}
	for i := range args.spenders {
		spender := args.spenders[i]
		receiver := args.receivers[i]
		contractSpender, ok1 := spender.GetContract()
		contractReceiver, ok2 := receiver.GetContract()
		if ok1 != ok2 {
			return nil, fmt.Errorf("spender is sending '%s' asset but receiver is receiving '%s' asset", contractSpender, contractReceiver)
		}
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

// func (args *MultiTransferArgs) TotalTransferAmount(contract *xc.ContractAddress) xc.AmountBlockchain {
// 	total := xc.NewAmountBlockchainFromUint64(0)
// 	for _, receiver := range args.receivers {
// 		expectedContract := contract != nil
// 		receiverContract, ok := receiver.GetContract()
// 		if !ok && !expectedContract {
// 			amount := receiver.GetAmount()
// 			total = total.Add(&amount)
// 			continue
// 		}

// 		if ok && expectedContract {
// 			if receiverContract != *contract {
// 				amount := receiver.GetAmount()
// 				total = total.Add(&amount)
// 				continue
// 			}
// 		}

// 		if contract != nil && receiverContract == *contract {
// 			amount := receiver.GetAmount()
// 			total = total.Add(&amount)
// 			continue
// 		}

// 		if contract == nil && receiverContract == "" {
// 			amount := receiver.GetAmount()
// 			total = total.Add(&amount)
// 			continue
// 		}
// 	}
// 	return total
// }
