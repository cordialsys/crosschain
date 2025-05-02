package tx

import (
	"encoding/hex"
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/cardano/client/types"
	"github.com/cordialsys/crosschain/chain/cardano/tx_input"
	"github.com/cosmos/btcutil/bech32"
	"github.com/fxamacker/cbor/v2"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/blake2b"
)

const (
	// PolicyId is 28 byte hash of the token policy
	PolicyIdLen = 56
	Lovelace    = "lovelace"
	FeeMargin   = 500
)

// Tx for Cardano
type Tx struct {
	_       struct{} `cbor:",toarray"`
	Body    *TxBody
	Witness *Witness
	Valid   bool
	Memo    *string
}

func newTx() *Tx {
	return &Tx{
		Body: &TxBody{
			Inputs:  make([]*Input, 0),
			Outputs: make([]*Output, 0),
		},
		Witness: &Witness{
			Keys: make([]*VKeyWitness, 0),
		},
		Valid: true,
	}
}

type TxBody struct {
	Inputs   []*Input  `cbor:"0,keyasint"`
	Outputs  []*Output `cbor:"1,keyasint"`
	Fee      uint64    `cbor:"2,keyasint"`
	TTL      uint32    `cbor:"3,keyasint,omitempty"`
	MemoHash []byte    `cbor:"7,keyasint,omitempty"`
}

func (txBody *TxBody) MarshalCBOR() ([]byte, error) {
	type Body struct {
		Inputs  cbor.Tag  `cbor:"0,keyasint"`
		Outputs []*Output `cbor:"1,keyasint"`
		Fee     uint64    `cbor:"2,keyasint"`
		TTL     uint32    `cbor:"3,keyasint,omitempty,omitzero"`
	}
	txBodyData := Body{
		Inputs: cbor.Tag{
			Number:  258,
			Content: txBody.Inputs,
		},
		Outputs: txBody.Outputs,
		Fee:     txBody.Fee,
		TTL:     txBody.TTL,
	}
	return cbor.Marshal(txBodyData)
}

type Input struct {
	_ struct{} `cbor:",toarray"`

	TxHash []byte
	Index  uint16
}

type PolicyHash [28]byte
type TokenNameHexToAmount map[string]uint64
type TokenAmounts struct {
	_                 struct{} `cbor:",toarray"`
	NativeAmount      uint64
	PolicyIdToAmounts map[PolicyHash]TokenNameHexToAmount
}

type Output struct {
	Address       []byte
	TokenAmounts  TokenAmounts
	NameLengthSum uint64
}

func DecodeToBase256(bech string) (string, []byte, error) {
	hrp, data, err := bech32.DecodeNoLimit(bech)
	if err != nil {
		return "", nil, err
	}

	converted, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return "", nil, err
	}
	return hrp, converted, nil
}

func NewOutput(address xc.Address) (*Output, error) {
	_, receiverAddressBytes, err := DecodeToBase256(string(address))
	if err != nil {
		return nil, fmt.Errorf("failed to decode address: %w", err)
	}

	return &Output{
		Address: receiverAddressBytes,
	}, nil
}

func (o *Output) MarshalCBOR() ([]byte, error) {
	nativeOutput := o.TokenAmounts.PolicyIdToAmounts == nil
	if nativeOutput {
		type output struct {
			_       struct{} `cbor:",toarray"`
			Address []byte
			Amount  uint64
		}
		nativeOutput := output{
			Address: o.Address,
			Amount:  o.TokenAmounts.NativeAmount,
		}
		return cbor.Marshal(nativeOutput)
	} else {
		type output struct {
			Address []byte       `cbor:"0,keyasint"`
			Amounts TokenAmounts `cbor:"1,keyasint"`
		}

		tokenOutput := output{
			Address: o.Address,
			Amounts: o.TokenAmounts,
		}

		return cbor.Marshal(tokenOutput)
	}
}

func (to *Output) AddAmount(amount uint64, contract xc.ContractAddress) error {
	if contract == Lovelace || contract == "" {
		to.TokenAmounts.NativeAmount = amount
		return nil
	}

	policyId := string(contract[:PolicyIdLen])
	assetName, err := hex.DecodeString(string(contract[PolicyIdLen:]))
	policyHashRaw, err := hex.DecodeString(policyId)
	if err != nil {
		return fmt.Errorf("failed to decode policyId: %w", err)
	}
	policyHash := PolicyHash(policyHashRaw)

	if to.TokenAmounts.PolicyIdToAmounts == nil {
		to.TokenAmounts.PolicyIdToAmounts = make(map[PolicyHash]TokenNameHexToAmount)
	}

	_, ok := to.TokenAmounts.PolicyIdToAmounts[policyHash]
	if !ok {
		to.TokenAmounts.PolicyIdToAmounts[policyHash] = make(TokenNameHexToAmount, 0)
	}
	tokenAmounts := to.TokenAmounts.PolicyIdToAmounts[policyHash]
	tokenAmounts[string(assetName)] = amount

	return nil
}

type Witness struct {
	Keys []*VKeyWitness `cbor:"0,keyasint,omitempty"`
}

func (w *Witness) MarshalCBOR() ([]byte, error) {
	type arrayWitness struct {
		Keys cbor.Tag `cbor:"0,keyasint,omitempty"`
	}
	witness := arrayWitness{
		Keys: cbor.Tag{
			Number:  258,
			Content: w.Keys,
		},
	}
	return cbor.Marshal(witness)
}

type VKeyWitness struct {
	_         struct{} `cbor:",toarray"`
	VKey      []byte
	Signature []byte
}

var _ xc.Tx = &Tx{}

func CalcMinUtxoValue(coinsPerUtxoWord uint64) xc.AmountBlockchain {
	return xc.NewAmountBlockchainFromUint64(0)
}

func CreateOutput(args xcbuilder.TransferArgs, input tx_input.TxInput) (*Output, error) {
	// Create first output
	receiverAddress := args.GetTo()
	output, err := NewOutput(receiverAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create output: %w", err)
	}

	amount := args.GetAmount()
	contract, ok := args.GetContract()
	isNative := !ok
	output.AddAmount(amount.Uint64(), contract)
	if !isNative {
		minLovelace := CalcMinUtxoValue(input.CoinsPerUtxoWord.Uint64())
		output.AddAmount(minLovelace.Uint64(), xc.ContractAddress(Lovelace))
	}

	return output, nil
}

// CreateChangeOutput creates a change output for the transaction
// 1. Calculate total contract amounts from utxos
// 2. Calculate total contract amounts from output
// 3. Calculate change amounts for both native and token outputs
// 4. Create change output
func CreateChangeOutput(utxos []types.Utxo, output *Output, args xcbuilder.TransferArgs) (*Output, error) {
	if output == nil {
		return nil, errors.New("invalid output")
	}

	// Calculate total input amounts
	inputAmounts := make(map[xc.ContractAddress]xc.AmountBlockchain)
	for _, utxo := range utxos {
		for _, amount := range utxo.Amounts {
			contract := xc.ContractAddress(amount.Unit)
			inputAmount, ok := inputAmounts[contract]
			if !ok {
				inputAmount = xc.NewAmountBlockchainFromUint64(0)
			}
			inputQuantity := xc.NewAmountBlockchainFromStr(amount.Quantity)
			inputAmounts[contract] = inputAmount.Add(&inputQuantity)
			log.WithFields(log.Fields{
				"contract": contract,
				"amount":   inputAmounts[contract],
			}).Debug("input amount")
		}
	}

	// Make sure that inputs contain lovelace
	_, ok := inputAmounts[Lovelace]
	if !ok {
		return nil, errors.New("missing lovelace input")
	}

	// Calculate total output amounts
	totalOutputs := make(map[xc.ContractAddress]xc.AmountBlockchain)
	// Include native output first
	nativeOutput := xc.NewAmountBlockchainFromUint64(output.TokenAmounts.NativeAmount)
	if nativeOutput.IsZero() {
		return nil, errors.New("outputs with 0 lovelace are invalid")
	}
	totalOutputs[Lovelace] = nativeOutput
	for policyId, amounts := range output.TokenAmounts.PolicyIdToAmounts {
		hexPolId := hex.EncodeToString(policyId[:])

		for assetName, tokenAmount := range amounts {
			contract := xc.ContractAddress(fmt.Sprintf("%s%s", hexPolId, assetName))
			totalOutputs[contract] = xc.NewAmountBlockchainFromUint64(tokenAmount)
		}
	}

	// Iterate over inputs and calculate change amounts
	changeOutput, err := NewOutput(args.GetFrom())
	if err != nil {
		return nil, fmt.Errorf("failed to create change output: %w", err)
	}
	zeroAmount := xc.NewAmountBlockchainFromUint64(0)
	for contract, inputAmount := range inputAmounts {
		outputAmount, ok := totalOutputs[contract]
		if !ok {
			outputAmount = zeroAmount
		}
		log.WithFields(log.Fields{
			"contract": contract,
			"amount":   outputAmount,
		}).Debug("output amount")

		changeAmount := inputAmount.Sub(&outputAmount)
		if changeAmount.Cmp(&zeroAmount) == -1 {
			return nil, fmt.Errorf("negative change amount for contract %s", contract)
		}
		changeOutput.AddAmount(changeAmount.Uint64(), contract)
		log.WithFields(log.Fields{
			"contract": contract,
			"amount":   changeAmount,
		}).Debug("change amount")
	}

	return changeOutput, nil
}

// Create Cardano transaction
// 1. Create raw transaction
// 2. Create inputs
// 3. Create recipient output
// 4. Create change output
// 5. Create dummy signature for fee estimation
// 6. Deduct fee from change output
func NewTx(args xcbuilder.TransferArgs, input tx_input.TxInput) (xc.Tx, error) {
	tx := newTx()

	inputMemo, ok := args.GetMemo()
	if ok {
		tx.Memo = &inputMemo
		bytes, err := cbor.Marshal(inputMemo)
		if err != nil {
			return nil, errors.New("failed to marshal memo")
		}
		hash := blake2b.Sum256(bytes)
		tx.Body.MemoHash = hash[:]
	}

	// Create inputs
	for _, utxo := range input.Utxos {
		hehHash, err := hex.DecodeString(utxo.TxHash)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hash: %w", err)
		}
		txInput := &Input{
			TxHash: hehHash,
			Index:  utxo.Index,
		}
		tx.Body.Inputs = append(tx.Body.Inputs, txInput)
	}

	// Recipient output
	output, err := CreateOutput(args, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create output: %w", err)
	}
	tx.Body.Outputs = append(tx.Body.Outputs, output)

	// Change output
	changeOutput, err := CreateChangeOutput(input.Utxos, output, args)
	if err != nil {
		return nil, fmt.Errorf("failed to create change output: %w", err)
	}
	tx.Body.Outputs = append(tx.Body.Outputs, changeOutput)

	tx.Body.TTL = uint32(input.Slot + 3600*2) // 2 hours

	// Witness for fee estimation
	dummyWitness := &VKeyWitness{
		VKey:      make([]byte, 32),
		Signature: make([]byte, 64),
	}
	tx.Witness.Keys = append(tx.Witness.Keys, dummyWitness)
	// Make sure to zero out the dummy witness
	defer func() {
		tx.Witness.Keys = make([]*VKeyWitness, 0)
	}()

	// Serialize tx with dummy signature for fee estimation
	txCbor, err := cbor.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tx: %w", err)
	}

	// Calculate fee
	txSize := xc.NewAmountBlockchainFromUint64(uint64(len(txCbor)))
	feeMargin := xc.NewAmountBlockchainFromUint64(FeeMargin)
	fee := input.FeePerByte
	fee = fee.Mul(&txSize)
	fee = fee.Add(&input.FixedFee)
	fee = fee.Add(&feeMargin)
	tx.Body.Fee = fee.Uint64()
	tx.Body.Outputs[1].TokenAmounts.NativeAmount -= tx.Body.Fee
	return tx, nil
}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	txCbor, err := cbor.Marshal(tx.Body)
	if err != nil {
		return xc.TxHash("")
	}

	blake2bHash := blake2b.Sum256(txCbor)

	return xc.TxHash(hex.EncodeToString(blake2bHash[:]))
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	txCbor, err := cbor.Marshal(tx.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tx: %w", err)
	}
	hash := blake2b.Sum256(txCbor)
	signatureData := &xc.SignatureRequest{
		Payload: hash[:],
	}
	println("Sighash: ", hex.EncodeToString(signatureData.Payload))

	return []*xc.SignatureRequest{signatureData}, nil
}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...*xc.SignatureResponse) error {
	if len(signatures) == 0 {
		return nil
	}

	if len(tx.Witness.Keys) != 0 {
		return errors.New("tx already signed")
	}

	for _, sig := range signatures {
		vKeyWitness := &VKeyWitness{
			VKey:      sig.PublicKey,
			Signature: sig.Signature,
		}
		tx.Witness.Keys = append(tx.Witness.Keys, vKeyWitness)
	}

	return nil
}

// GetSignatures returns back signatures, which may be used for signed-transaction broadcasting
func (tx *Tx) GetSignatures() []xc.TxSignature {
	signatures := make([]xc.TxSignature, len(tx.Witness.Keys))
	for _, witness := range tx.Witness.Keys {
		signatures = append(signatures, xc.TxSignature(witness.Signature))
	}
	return signatures
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	txCbor, err := cbor.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tx: %w", err)
	}

	return txCbor, nil
}
