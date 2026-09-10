package tempo

import (
	"math/big"
	"strings"

	xc "github.com/cordialsys/crosschain"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

type TxInput struct {
	evminput.TxInput
	FeeContract xc.ContractAddress `json:"fee_contract,omitempty"`
	// Native Tempo sponsorship does not consume a fee-payer nonce.
	NativeFeePayer bool `json:"native_fee_payer,omitempty"`
}

var _ xc.TxInput = &TxInput{}
var _ evminput.GetAccountInfo = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	input := evminput.NewTxInput()
	input.Type = xc.DriverTempo
	return &TxInput{
		TxInput: *input,
	}
}

func NewTxInputFromEVM(input *evminput.TxInput, feeContract xc.ContractAddress) *TxInput {
	input.Type = xc.DriverTempo
	return &TxInput{
		TxInput:     *input,
		FeeContract: feeContract,
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverTempo
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	return input.TxInput.SetGasFeePriority(other)
}
func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	if input.NativeFeePayer {
		account, ok := other.(evminput.GetAccountInfo)
		if !ok {
			return false
		}
		if strings.EqualFold(input.GetFromAddress(), account.GetFromAddress()) && input.Nonce == account.GetNonce() {
			return false
		}
		// An EIP-7702 transaction can consume our sender's nonce as its payer.
		otherTempo, native := other.(*TxInput)
		if !(native && otherTempo.NativeFeePayer) && strings.EqualFold(input.GetFromAddress(), account.GetFeePayerAddress()) && input.Nonce == account.GetFeePayerNonce() {
			return false
		}
		return true
	}
	return input.TxInput.IndependentOf(other)
}
func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (independent bool) {
	if input.NativeFeePayer {
		return other != nil && !input.IndependentOf(other)
	}
	return input.TxInput.SafeFromDoubleSend(other)
}
func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	// Tempo reports fee pricing with extra precision, while fee limits are
	// compared in the TIP-20 contract precision. Round up so inclusive-fee
	// sweeps reserve enough balance for the fee token before the token transfer
	// executes.
	attoTempo := xc.NewAmountBlockchainFromUint64(1_000_000_000_000)
	feeLimit, contract := input.TxInput.GetFeeLimit()
	if contract == "" {
		contract = input.FeeContract
	}
	return divCeil(feeLimit, attoTempo), contract
}

func divCeil(amount xc.AmountBlockchain, divisor xc.AmountBlockchain) xc.AmountBlockchain {
	quotient, remainder := new(big.Int).QuoRem(amount.Int(), divisor.Int(), new(big.Int))
	if remainder.Sign() != 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return xc.AmountBlockchain(*quotient)
}

func (input *TxInput) GetFeePayerAddress() string {
	return input.TxInput.GetFeePayerAddress()
}
func (input *TxInput) GetFeePayerNonce() uint64 {
	return input.TxInput.GetFeePayerNonce()
}
func (input *TxInput) GetFromAddress() string {
	return input.TxInput.GetFromAddress()
}
func (input *TxInput) GetNonce() uint64 {
	return input.TxInput.GetNonce()
}

func (input *TxInput) IsFeeLimitAccurate() bool {
	if input.NativeFeePayer {
		return false
	}
	return input.TxInput.IsFeeLimitAccurate()
}

func init() {
	registry.RegisterTxVariantInput(&CallInput{})
}

type CallInput struct {
	// base tx input
	TxInput
	// no additional info is needed for evm call currently
}

var _ xc.TxVariantInput = &CallInput{}
var _ xc.CallTxInput = &CallInput{}

func NewCallInput() *CallInput {
	return &CallInput{}
}

func (*CallInput) GetVariant() xc.TxVariantInputType {
	return xc.NewCallingInputType(xc.DriverTempo)
}

// Mark as valid for calling transactions
func (*CallInput) Calling() {}

func (input *CallInput) GetNonce() uint64 {
	return input.Nonce
}

func (input *CallInput) GetFromAddress() string {
	return string(input.FromAddress)
}

func (input *CallInput) GetFeePayerNonce() uint64 {
	return input.FeePayerNonce
}

func (input *CallInput) GetFeePayerAddress() string {
	return string(input.FeePayerAddress)
}

type MultiTransferInput struct {
	TxInput
}

func init() {
	registry.RegisterTxVariantInput(&MultiTransferInput{})
}

var _ xc.TxVariantInput = &MultiTransferInput{}
var _ xc.MultiTransferInput = &MultiTransferInput{}

func NewMultiTransferInput() *MultiTransferInput {
	return &MultiTransferInput{}
}

func (input *MultiTransferInput) GetVariant() xc.TxVariantInputType {
	return xc.NewMultiTransferInputType(xc.DriverTempo, "eip7702")
}

func (input *MultiTransferInput) MultiTransfer() {}

func (input *MultiTransferInput) GetNonce() uint64 {
	return input.Nonce
}

func (input *MultiTransferInput) GetFromAddress() string {
	return string(input.FromAddress)
}

func (input *MultiTransferInput) GetFeePayerNonce() uint64 {
	return input.FeePayerNonce
}

func (input *MultiTransferInput) GetFeePayerAddress() string {
	return string(input.FeePayerAddress)
}
