package builder

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	icpaddress "github.com/cordialsys/crosschain/chain/internet_computer/address"
)

type TransferArgs struct {
	// TxCommonOptions
	appliedOptions []BuilderOption
	options        builderOptions
	from           xc.Address
	to             xc.Address
	amount         xc.AmountBlockchain
}

var _ TransactionOptions = &TransferArgs{}

// Transfer relevant arguments
func (args *TransferArgs) GetFrom() xc.Address            { return args.from }
func (args *TransferArgs) GetTo() xc.Address              { return args.to }
func (args *TransferArgs) GetAmount() xc.AmountBlockchain { return args.amount }

// Exposed options
func (args *TransferArgs) GetMemo() (string, bool)                { return args.options.GetMemo() }
func (args *TransferArgs) GetTimestamp() (int64, bool)            { return args.options.GetTimestamp() }
func (args *TransferArgs) GetPriority() (xc.GasFeePriority, bool) { return args.options.GetPriority() }
func (args *TransferArgs) GetPublicKey() ([]byte, bool)           { return args.options.GetPublicKey() }
func (args *TransferArgs) GetContract() (xc.ContractAddress, bool) {
	return args.options.GetContract()
}

func (args *TransferArgs) GetFeePayer() (xc.Address, bool) {
	return args.options.GetFeePayer()
}

func (args *TransferArgs) GetFeePayerPublicKey() ([]byte, bool) {
	return args.options.GetFeePayerPublicKey()
}

// Decimals for token contract, which may be needed for token transfers on some chains
func (args *TransferArgs) GetDecimals() (int, bool) {
	return args.options.GetDecimals()
}

func (args *TransferArgs) InclusiveFeeSpendingEnabled() bool {
	return args.options.InclusiveFeeSpendingEnabled()
}

func (args *TransferArgs) GetTransactionAttempts() []string {
	return args.options.GetTransactionAttempts()
}

func (args *TransferArgs) GetFromIdentity() (string, bool) {
	return args.options.GetFromIdentity()
}

func (args *TransferArgs) GetFeePayerIdentity() (string, bool) {
	return args.options.GetFeePayerIdentity()
}

func (args *TransferArgs) GetToIdentity() (string, bool) {
	return args.options.GetToIdentity()
}

func NewTransferArgs(chain *xc.ChainBaseConfig, from xc.Address, to xc.Address, amount xc.AmountBlockchain, options ...BuilderOption) (TransferArgs, error) {
	builderOptions := newBuilderOptions()
	appliedOptions := options
	args := TransferArgs{
		appliedOptions,
		builderOptions,
		from,
		to,
		amount,
	}
	for _, opt := range options {
		err := opt(&args.options)
		if err != nil {
			return args, err
		}
	}

	switch chain.Driver {
	case xc.DriverInternetComputerProtocol:
		fromFormat, fromOk := icpaddress.GetAddressType(from)
		toFormat, toOk := icpaddress.GetAddressType(to)
		if fromOk && toOk && fromFormat != toFormat {
			return args, fmt.Errorf("can only send between addresses of the same type for ICP")
		}
	}

	return args, nil
}

func (args *TransferArgs) SetAmount(amount xc.AmountBlockchain) {
	args.amount = amount
}

func (args *TransferArgs) SetFrom(from xc.Address) {
	args.from = from
}

func (args *TransferArgs) SetTo(to xc.Address) {
	args.to = to
}

func (args *TransferArgs) SetContract(contract xc.ContractAddress) {
	args.options.SetContract(contract)
}

func (args *TransferArgs) SetPublicKey(publicKey []byte) {
	args.options.publicKey = &publicKey
}

func (args *TransferArgs) SetFeePayer(feePayer xc.Address) {
	args.options.SetFeePayer(feePayer)
}

func (args *TransferArgs) SetFeePayerPublicKey(feePayerPublicKey []byte) {
	args.options.SetFeePayerPublicKey(feePayerPublicKey)
}

func (args *TransferArgs) SetInclusiveFeeSpending(inclusiveFeeSpending bool) {
	args.options.SetInclusiveFeeSpending(inclusiveFeeSpending)
}

func (args *TransferArgs) SetTransactionAttempts(previousTransactionAttempts []string) {
	args.options.SetTransactionAttempts(previousTransactionAttempts)
}

func (args *TransferArgs) SetFromIdentity(fromIdentity string) {
	args.options.SetFromIdentity(fromIdentity)
}

func (args *TransferArgs) SetToIdentity(toIdentity string) {
	args.options.SetToIdentity(toIdentity)
}
