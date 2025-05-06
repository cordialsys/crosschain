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
	PolicyIdLen             = 56
	FeeMargin               = 500
	DefaultHeaderSize       = 6
	PolicyIdSize            = 28
	AssetNameOverhead       = 12
	UtxoEntrySizeWithoutVal = 27
	CoinsPerUtxoWord        = 37_037
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

func ContractAddressToPolicyAndName(contract xc.ContractAddress) (PolicyHash, TokenName) {
	policyId := string(contract[:PolicyIdLen])
	assetName := string(contract[PolicyIdLen:])
	return PolicyHash(policyId), TokenName(assetName)
}

type PolicyHash string

func (p *PolicyHash) MarshalCBOR() ([]byte, error) {
	policyBytes, err := hex.DecodeString(string(*p))
	if err != nil {
		return nil, fmt.Errorf("failed to decode policyId: %w", err)
	}
	return cbor.Marshal(policyBytes)
}

type TokenName string

// TokenName should be encoded as CBOR byte string, followed by the length of the string
func (t *TokenName) MarshalCBOR() ([]byte, error) {
	// AssetName is hex encoded in RPC/cli requests, but decoded on chain
	strName, err := hex.DecodeString(string(*t))
	if err != nil {
		return nil, fmt.Errorf("failed to decode token name: %w", err)
	}
	length := uint8(len(strName))
	// Tag as ByteString - fxamacker/cbor ByteString array is serializing to byte array instead
	bytes := []byte{0x58}
	bytes = append(bytes, length)
	bytes = append(bytes, []byte(strName)...)
	return bytes, nil
}

type TokenNameHexToAmount map[TokenName]uint64

type TokenAmounts struct {
	_                 struct{} `cbor:",toarray"`
	NativeAmount      uint64
	PolicyIdToAmounts map[PolicyHash]TokenNameHexToAmount
}

func (ta *TokenAmounts) AddAmount(contract xc.ContractAddress, amount uint64) {
	if contract == types.Lovelace || contract == "" {
		ta.NativeAmount = amount
		return
	}

	policyId, assetName := ContractAddressToPolicyAndName(contract)
	if ta.PolicyIdToAmounts == nil {
		ta.PolicyIdToAmounts = make(map[PolicyHash]TokenNameHexToAmount)
	}

	_, ok := ta.PolicyIdToAmounts[policyId]
	if !ok {
		ta.PolicyIdToAmounts[policyId] = make(TokenNameHexToAmount, 0)
	}
	tokenAmounts := ta.PolicyIdToAmounts[policyId]
	tokenAmounts[assetName] = amount
}

func (ta *TokenAmounts) GetAmount(policyId PolicyHash, tokenName TokenName) uint64 {
	policyAssets, ok := ta.PolicyIdToAmounts[policyId]
	if ok {
		tokenAmount, ok := policyAssets[tokenName]
		if ok {
			return tokenAmount
		}
	}

	return 0
}

// Returns true when `ta` can cover all `required` amounts
func (ta TokenAmounts) CanCover(required TokenAmounts) bool {
	for policyId, amounts := range required.PolicyIdToAmounts {
		for assetName, requiredAmount := range amounts {
			amount := ta.GetAmount(policyId, assetName)
			if amount < requiredAmount {
				return false
			}
		}
	}
	return true
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
			_       struct{} `cbor:",toarray"`
			Address []byte
			Amounts TokenAmounts
		}

		tokenOutput := output{
			Address: o.Address,
			Amounts: o.TokenAmounts,
		}

		return cbor.Marshal(tokenOutput)
	}
}

func (to *Output) AddAmount(contract xc.ContractAddress, amount uint64) {
	to.TokenAmounts.AddAmount(contract, amount)
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

// CalcMinAdaValue calculates the minimum UTXO value for a given set of token amounts
// based on the Cardano protocol parameters.
// Base formula: https://cardano-ledger.readthedocs.io/en/latest/explanations/min-utxo-mary.html
// Alonzo follow up: https://github.com/IntersectMBO/cardano-ledger/blob/master/doc/explanations/min-utxo-alonzo.rst
func CalcMinAdaValue(policyHashToAmounts map[PolicyHash]TokenNameHexToAmount) xc.AmountBlockchain {
	policyIdCount := len(policyHashToAmounts)

	// Count assets and total asset name characters
	totalCharCount := 0
	assetCount := 0
	for _, assetAmounts := range policyHashToAmounts {
		for assetName, _ := range assetAmounts {
			assetCount += 1
			totalCharCount += len(assetName)
		}
	}

	policyMultiplier := 0
	if policyIdCount > assetCount {
		policyMultiplier = policyIdCount
	} else {
		policyMultiplier = assetCount
	}

	sizeOfValue := (policyMultiplier * AssetNameOverhead) + totalCharCount + (policyIdCount * PolicyIdSize)

	// Round up to bytes
	sizeOfValue = (sizeOfValue + 7) / 8
	// Add overhead for the UTXO entry
	sizeOfValue += DefaultHeaderSize

	utxoEntrySize := xc.NewAmountBlockchainFromUint64(uint64(UtxoEntrySizeWithoutVal + sizeOfValue))
	coinsPerUtxoWord := xc.NewAmountBlockchainFromUint64(uint64(CoinsPerUtxoWord))
	utxoEntrySize = utxoEntrySize.Mul(&coinsPerUtxoWord)
	adamount := xc.NewAmountBlockchainFromUint64(1500000)
	return utxoEntrySize.Add(&adamount) // 1.5 ADA
}

func CreateOutput(args xcbuilder.TransferArgs, input tx_input.TxInput) (*Output, error) {
	// Create firt output
	receiverAddress := args.GetTo()
	output, err := NewOutput(receiverAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create output: %w", err)
	}

	amount := args.GetAmount()
	contract, ok := args.GetContract()
	isNative := !ok
	output.AddAmount(contract, amount.Uint64())
	if !isNative {
		minLovelace := CalcMinAdaValue(output.TokenAmounts.PolicyIdToAmounts)
		output.AddAmount(xc.ContractAddress(types.Lovelace), minLovelace.Uint64())
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
	_, ok := inputAmounts[types.Lovelace]
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
	totalOutputs[types.Lovelace] = nativeOutput
	for policyId, amounts := range output.TokenAmounts.PolicyIdToAmounts {
		for assetName, tokenAmount := range amounts {
			contract := xc.ContractAddress(string(policyId) + string(assetName))
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
		changeOutput.AddAmount(contract, changeAmount.Uint64())
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
		hexHash, err := hex.DecodeString(utxo.TxHash)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hash: %w", err)
		}
		txInput := &Input{
			TxHash: hexHash,
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

	// TX will be valid indefinitely if TTL is not set
	if input.TransactionValidityTime > 0 {
		tx.Body.TTL = uint32(input.Slot + input.TransactionValidityTime) // 2 hours
	}
	tx.Body.Fee = input.Fee
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

	return []*xc.SignatureRequest{signatureData}, nil
}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...*xc.SignatureResponse) error {
	if len(signatures) == 0 {
		return errors.New("no signatures provided")
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
