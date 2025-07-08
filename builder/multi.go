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

func NewMultiTransferArgs(chain xc.NativeAsset, spenders []*Sender, receivers []*Receiver, options ...BuilderOption) (*MultiTransferArgs, error) {
	builderOptions := newBuilderOptions()
	for _, opt := range options {
		err := opt(&builderOptions)
		if err != nil {
			return nil, err
		}
	}
	switch chain.Driver() {
	case xc.DriverBitcoin, xc.DriverBitcoinCash, xc.DriverBitcoinLegacy, xc.DriverCardano, xc.DriverSui:
		// check for address dups
		for _, s1 := range spenders {
			for _, s2 := range spenders {
				if s1.address == s2.address {
					return nil, errors.New("cannot use the same address multiple times in a batch transaction for a UTXO-based chain")
				}
			}
		}

	case xc.DriverEVM, xc.DriverEVMLegacy:
		if len(spenders) != 1 {
			return nil, errors.New("only one spender is supported for account-based chains")
		}
		_, ok := builderOptions.GetFeePayer()
		if !ok {
			return nil, errors.New("separate fee-payer must be set for multi-transfers on EVM-based chains")
		}
	case xc.DriverSolana:
		if len(spenders) != 1 {
			return nil, errors.New("only one spender is supported for account-based chains")
		}
	}
	return &MultiTransferArgs{
		spenders,
		receivers,
		builderOptions,
		options,
	}, nil
}

func NewMultiTransferArgsFromSingle(chain xc.NativeAsset, single *TransferArgs, options ...BuilderOption) (*MultiTransferArgs, error) {
	senderPublicKey, ok := single.GetPublicKey()
	if !ok {
		return nil, errors.New("sender public key not set")
	}
	sender, err := NewSender(single.GetFrom(), senderPublicKey, options...)
	if err != nil {
		return nil, err
	}
	receiver, err := NewReceiver(single.GetTo(), single.GetAmount(), options...)
	if err != nil {
		return nil, err
	}
	// forkt the options from the single transfer args
	appliedOptions := single.appliedOptions
	appliedOptions = append(appliedOptions, options...)

	multi, err := NewMultiTransferArgs(chain, []*Sender{sender}, []*Receiver{receiver}, appliedOptions...)
	if err != nil {
		return nil, err
	}
	return multi, nil
}

func (args *Sender) GetFrom() xc.Address { return args.address }

func (args *Sender) GetPublicKey() []byte {
	return args.publicKey
}

func (args *Sender) GetFromIdentity() (string, bool) {
	return args.options.GetFromIdentity()
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

func (args *MultiTransferArgs) SetFeePayer(feePayer xc.Address) {
	args.options.SetFeePayer(feePayer)
}

func (args *MultiTransferArgs) GetFeePayerPublicKey() ([]byte, bool) {
	return args.options.GetFeePayerPublicKey()
}

func (args *MultiTransferArgs) GetFeePayerIdentity() (string, bool) {
	return args.options.GetFeePayerIdentity()
}

func (args *MultiTransferArgs) GetMemo() (string, bool) {
	return args.options.GetMemo()
}

func (args *MultiTransferArgs) GetTransactionAttempts() []string {
	return args.options.GetTransactionAttempts()
}

func (args *MultiTransferArgs) AsUtxoTransfers() ([]*TransferArgs, error) {
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

func (args *MultiTransferArgs) AsAccountTransfers() ([]*TransferArgs, error) {
	if len(args.spenders) != 1 {
		return nil, errors.New("can only be one spender for an account-based chain")
	}
	transfers := make([]*TransferArgs, len(args.receivers))
	spender := args.spenders[0]
	for i := range args.receivers {
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
