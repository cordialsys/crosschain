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
	// Native sponsorship does not consume FeePayerNonce. Older EIP-7702 inputs do.
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
	known, conflict := input.nonceConflict(other)
	return known && !conflict
}
func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (independent bool) {
	known, conflict := input.nonceConflict(other)
	return known && conflict
}

// This method is promoted by Tempo call and multi-transfer input wrappers.
func (input *TxInput) nativeFeePayer() bool { return input.NativeFeePayer }

type accountNonce struct {
	address string
	nonce   uint64
}

func consumedNonces(input evminput.GetAccountInfo) []accountNonce {
	nonces := []accountNonce{{input.GetFromAddress(), input.GetNonce()}}
	native, ok := input.(interface{ nativeFeePayer() bool })
	if input.GetFeePayerAddress() != "" && !(ok && native.nativeFeePayer()) {
		nonces = append(nonces, accountNonce{input.GetFeePayerAddress(), input.GetFeePayerNonce()})
	}
	return nonces
}

func (input *TxInput) nonceConflict(other xc.TxInput) (known, conflict bool) {
	account, ok := other.(evminput.GetAccountInfo)
	if !ok || input.GetFromAddress() == "" || account.GetFromAddress() == "" {
		return false, false
	}
	// Compare both sets symmetrically, including the EIP-7702 payer nonce when
	// present. A shared native sponsor does not make transfers conflict.
	for _, a := range consumedNonces(input) {
		for _, b := range consumedNonces(account) {
			if a.nonce == b.nonce && strings.EqualFold(a.address, b.address) {
				return true, true
			}
		}
	}
	return true, false
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
	return !input.NativeFeePayer && input.TxInput.IsFeeLimitAccurate()
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
