package tx

import (
	"encoding/hex"
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	builder "github.com/cordialsys/crosschain/builder"
	buildererrors "github.com/cordialsys/crosschain/builder/errors"
	"github.com/cordialsys/crosschain/chain/cardano/address"
	"github.com/cordialsys/crosschain/chain/cardano/client/types"
	"github.com/cordialsys/crosschain/chain/cardano/tx_input"
	"github.com/cosmos/btcutil/bech32"
	"github.com/fxamacker/cbor/v2"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/btree"
	"golang.org/x/crypto/blake2b"
)

const (
	StakeCredentialKeyHash            = 0
	StakeCredentialScriptHash         = 1
	CertTypeDeregistration            = 8
	CertTypeRegistrationAndDelegation = 11
	// PolicyId is 28 byte hash of the token policy
	PolicyIdLen = 56

	// CalcMinAdaValue parameters, defined here: https://github.com/IntersectMBO/cardano-ledger/blob/master/doc/explanations/min-utxo-alonzo.rst
	// Some names map directly to the formula, while others are speculative on my part, especially if the name isn't clearly derived from the formula above.

	// Size overhead for an empty asset name
	AssetNameCountMultiplier = 12
	// Multiplier for the final size of the UTXO.
	CoinsPerUtxoWord = 37_037
	// Nameless in above document; however it's probably the size necessary to store ADA tokens
	NativeTokensHeaderSize = 6
	// Policy size is always a 28 byte hash
	PolicyIdSize = 28
	// Size of the UTXO entry without any coin values
	UtxoEntrySizeWithoutVal = 27
)

type CertificateCredential struct {
	_ struct{} `cbor:",toarray"`
	// Either stake verification key hash or script hash
	Type       uint32
	Credential []byte
}

func NewKeyCredential(pubkey []byte) (CertificateCredential, error) {
	keyHash, err := address.GetKeyHash(pubkey)
	if err != nil {
		return CertificateCredential{}, fmt.Errorf("failed to get keyhash: %w", err)
	}
	return CertificateCredential{
		Type:       StakeCredentialKeyHash,
		Credential: keyHash,
	}, nil
}

type Certificate struct {
	_                 struct{} `cbor:",toarray"`
	CertificationType uint32
	Credential        CertificateCredential
	PoolId            []byte `cbor:",omitempty,omitzero"`
	DepositAmount     uint64
}

func (c *Certificate) MarshalCBOR() ([]byte, error) {
	arr := make([]interface{}, 0)
	arr = append(arr, c.CertificationType)
	arr = append(arr, c.Credential)
	if c.PoolId != nil && len(c.PoolId) != 0 {
		arr = append(arr, c.PoolId)
	}
	arr = append(arr, c.DepositAmount)
	return cbor.Marshal(arr)
}

type Withdrawal struct {
	StakeAddress []byte
	Amount       uint64
}

// Serialize as map of address to amount
func (w *Withdrawal) MarshalCBOR() ([]byte, error) {
	data := make([]byte, 0)
	data = append(data, 0xA1) // CBOR map tag
	addressCbor, err := cbor.Marshal(w.StakeAddress)
	if err != nil {
		return nil, err
	}
	data = append(data, addressCbor...)

	amountCbor, err := cbor.Marshal(w.Amount)
	data = append(data, amountCbor...)
	return data, err
}

// Tx for Cardano
type Tx struct {
	_                           struct{} `cbor:",toarray"`
	Body                        *TxBody
	Witness                     *Witness
	Valid                       bool
	Memo                        *string
	RequiresAdditionalSignature bool `cbor:"-"`
}

func NewTx() *Tx {
	return &Tx{
		Body: &TxBody{
			Inputs:     make([]*Input, 0),
			Outputs:    make([]*Output, 0),
			Withdrawal: nil,
		},
		Witness: &Witness{
			Keys: make([]*VKeyWitness, 0),
		},
		Valid: true,
	}
}

type TxBody struct {
	Inputs       []*Input      `cbor:"0,keyasint"`
	Outputs      []*Output     `cbor:"1,keyasint"`
	Fee          uint64        `cbor:"2,keyasint"`
	TTL          uint32        `cbor:"3,keyasint,omitempty"`
	Certificates []Certificate `cbor:"4,keyasint,omitempty"`
	Withdrawal   *Withdrawal   `cbor:"5,keyasint,omitempty"`
	MemoHash     []byte        `cbor:"7,keyasint,omitempty"`
}

func (txBody *TxBody) MarshalCBOR() ([]byte, error) {
	type Body struct {
		Inputs       cbor.Tag    `cbor:"0,keyasint"`
		Outputs      []*Output   `cbor:"1,keyasint"`
		Fee          uint64      `cbor:"2,keyasint"`
		TTL          uint32      `cbor:"3,keyasint,omitempty,omitzero"`
		Certificates *cbor.Tag   `cbor:"4,keyasint,omitempty"`
		Withdrawal   *Withdrawal `cbor:"5,keyasint,omitempty"`
	}
	txBodyData := Body{
		Inputs: cbor.Tag{
			Number:  258,
			Content: txBody.Inputs,
		},
		Outputs:    txBody.Outputs,
		Fee:        txBody.Fee,
		TTL:        txBody.TTL,
		Withdrawal: txBody.Withdrawal,
	}
	if len(txBody.Certificates) > 0 {
		txBodyData.Certificates = &cbor.Tag{
			Number:  258,
			Content: txBody.Certificates,
		}
	}
	return cbor.Marshal(txBodyData)
}

type Input struct {
	_ struct{} `cbor:",toarray"`

	TxHash []byte
	Index  uint16
}

func ContractAddressToPolicyAndName(contract xc.ContractAddress) (PolicyHash, TokenName) {
	if contract == "" || contract == types.Lovelace {
		return "", ""
	}
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

type ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64 | ~string
}

func MarshalCBORBtreeMap[key ordered, value any](m *btree.Map[key, value]) ([]byte, error) {
	// Append raw `object` CBOR header
	mapBytes := []byte{0xA0}
	// Ensure that map size is less than 15 - shouldn't happend, but we have to check
	len := m.Len()
	if len > 0x0000_1111 {
		return nil, fmt.Errorf("map size is too large: %d", len)
	}
	mapBytes[0] |= byte(len)

	iter := m.Iter()
	for ok := iter.First(); ok; ok = iter.Next() {
		key := iter.Key()
		keyCbor, err := cbor.Marshal(key)
		if err != nil {
			return nil, fmt.Errorf("failed to marshall key: %w", err)
		}
		mapBytes = append(mapBytes, keyCbor...)

		value := iter.Value()
		valueCbor, err := cbor.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal value: %w", err)
		}
		mapBytes = append(mapBytes, valueCbor...)
	}
	return mapBytes, nil
}

type TokenNameHexToAmount struct {
	*btree.Map[TokenName, uint64]
}

func (t *TokenNameHexToAmount) MarshalCBOR() ([]byte, error) {
	bytes, err := MarshalCBORBtreeMap(t.Map)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token name hex to amount: %w", err)
	}
	return bytes, nil
}

type PolicyIdToAmounts struct {
	*btree.Map[PolicyHash, TokenNameHexToAmount]
}

func (p *PolicyIdToAmounts) Iter() btree.MapIter[PolicyHash, TokenNameHexToAmount] {
	if p == nil {
		p = &PolicyIdToAmounts{btree.NewMap[PolicyHash, TokenNameHexToAmount](1)}
	}
	return p.Map.Iter()
}

func (p *PolicyIdToAmounts) MarshalCBOR() ([]byte, error) {
	bytes, err := MarshalCBORBtreeMap(p.Map)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token name hex to amount: %w", err)
	}
	return bytes, nil
}

type TokenAmounts struct {
	_                 struct{} `cbor:",toarray"`
	NativeAmount      uint64
	PolicyIdToAmounts *PolicyIdToAmounts `cbor:"1,keyasint,omitempty"`
}

func (ta *TokenAmounts) AddAmount(contract xc.ContractAddress, amount uint64) {
	if contract == types.Lovelace || contract == "" {
		ta.NativeAmount = amount
		return
	}

	policyId, assetName := ContractAddressToPolicyAndName(contract)
	if ta.PolicyIdToAmounts == nil {
		ta.PolicyIdToAmounts = &PolicyIdToAmounts{btree.NewMap[PolicyHash, TokenNameHexToAmount](1)}
	}

	_, ok := ta.PolicyIdToAmounts.Get(policyId)
	if !ok {
		ta.PolicyIdToAmounts.Set(
			policyId,
			TokenNameHexToAmount{btree.NewMap[TokenName, uint64](1)},
		)
	}

	// We just inserted a new policyId, skip check
	tokenAmounts, _ := ta.PolicyIdToAmounts.Get(policyId)

	assetAmounts, ok := tokenAmounts.Get(assetName)
	if ok {
		assetAmounts += amount
	} else {
		tokenAmounts.Set(assetName, amount)
	}
}

func (ta *TokenAmounts) GetAmount(policyId PolicyHash, tokenName TokenName) uint64 {
	if policyId == "" {
		return ta.NativeAmount
	}

	policyAssets, ok := ta.PolicyIdToAmounts.Get(policyId)
	if ok {
		tokenAmount, ok := policyAssets.Get(tokenName)
		if ok {
			return tokenAmount
		}
	}

	return 0
}

// Returns true when `ta` can cover all `required` amounts
func (ta TokenAmounts) CanCover(required TokenAmounts) bool {
	// Cannot cover native amount
	if ta.NativeAmount < required.NativeAmount {
		return false
	}
	// Can cover native, and no token amounts
	if ta.PolicyIdToAmounts == nil || required.PolicyIdToAmounts == nil {
		return true
	}

	iter := required.PolicyIdToAmounts.Iter()
	for ok := iter.First(); ok; ok = iter.Next() {
		policyId := iter.Key()
		amounts := iter.Value()

		assetsIter := amounts.Iter()
		for ok := assetsIter.First(); ok; ok = assetsIter.Next() {
			assetName := assetsIter.Key()
			requiredAmount := assetsIter.Value()
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
func CalcMinAdaValue(policyHashToAmounts *PolicyIdToAmounts) xc.AmountBlockchain {
	policyIdCount := policyHashToAmounts.Len()

	// Count assets and total asset name characters
	totalCharCount := 0
	assetCount := 0
	policyIter := policyHashToAmounts.Iter()
	for ok := policyIter.First(); ok; ok = policyIter.Next() {
		assetAmounts := policyIter.Value()
		assetIter := assetAmounts.Iter()
		for ok := assetIter.First(); ok; ok = assetIter.Next() {
			assetName := assetIter.Key()
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

	sizeOfValue := (policyMultiplier * AssetNameCountMultiplier) + totalCharCount + (policyIdCount * PolicyIdSize)

	// Round up to bytes
	sizeOfValue = (sizeOfValue + 7) / 8
	// Add overhead for the UTXO entry
	sizeOfValue += NativeTokensHeaderSize

	utxoEntrySize := xc.NewAmountBlockchainFromUint64(uint64(UtxoEntrySizeWithoutVal + sizeOfValue))
	coinsPerUtxoWord := xc.NewAmountBlockchainFromUint64(uint64(CoinsPerUtxoWord))
	utxoEntrySize = utxoEntrySize.Mul(&coinsPerUtxoWord)
	return utxoEntrySize
}

// Create output that represents amount sent to receiver address
func (tx Tx) CreateTransferOutput(to xc.Address, amount xc.AmountBlockchain, contract xc.ContractAddress) error {
	receiverAddress := to
	output, err := NewOutput(receiverAddress)
	if err != nil {
		return fmt.Errorf("failed to create transfer output: %w", err)
	}

	isNative := contract == "" || contract == types.Lovelace
	output.AddAmount(contract, amount.Uint64())
	if !isNative {
		minLovelace := CalcMinAdaValue(output.TokenAmounts.PolicyIdToAmounts)
		output.AddAmount(xc.ContractAddress(types.Lovelace), minLovelace.Uint64())
	}

	tx.Body.Outputs = append(tx.Body.Outputs, output)
	return nil
}

// CreateChangeOutput creates a change output for the transaction
// 1. Calculate total contract amounts from utxos
// 2. Calculate total contract amounts from output
// 3. Calculate change amounts for both native and token outputs
// 4. Create change output
func (tx Tx) CreateChangeOutput(utxos []types.Utxo, from xc.Address) error {
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
		}
	}

	// Make sure that inputs contain lovelace
	_, ok := inputAmounts[types.Lovelace]
	if !ok {
		return errors.New("missing lovelace input")
	}

	// Calculate total output amounts
	totalOutputs := make(map[xc.ContractAddress]xc.AmountBlockchain)
	for _, output := range tx.Body.Outputs {
		// Include native output first
		nativeOutput := xc.NewAmountBlockchainFromUint64(output.TokenAmounts.NativeAmount)
		if nativeOutput.IsZero() {
			return errors.New("outputs with 0 lovelace are invalid")
		}
		totalOutputs[types.Lovelace] = nativeOutput
		policyIter := output.TokenAmounts.PolicyIdToAmounts.Iter()
		for ok := policyIter.First(); ok; ok = policyIter.Next() {
			policyId := policyIter.Key()
			amounts := policyIter.Value()
			amountsIter := amounts.Iter()
			for ok := amountsIter.First(); ok; ok = amountsIter.Next() {
				assetName := amountsIter.Key()
				tokenAmount := amountsIter.Value()
				contract := xc.ContractAddress(string(policyId) + string(assetName))
				totalOutputs[contract] = xc.NewAmountBlockchainFromUint64(tokenAmount)
			}
		}
	}

	// Iterate over inputs and calculate change amounts
	changeOutput, err := NewOutput(from)
	if err != nil {
		return fmt.Errorf("failed to create change output: %w", err)
	}
	zeroAmount := xc.NewAmountBlockchainFromUint64(0)
	for contract, inputAmount := range inputAmounts {
		outputAmount, ok := totalOutputs[contract]
		if !ok {
			outputAmount = zeroAmount
		}
		changeAmount := inputAmount.Sub(&outputAmount)
		if changeAmount.Cmp(&zeroAmount) == -1 {
			return fmt.Errorf("negative change amount for contract %s", contract)
		}
		changeOutput.AddAmount(contract, changeAmount.Uint64())
	}

	if log.GetLevel() == log.DebugLevel {
		DebugAmounts(inputAmounts, totalOutputs, changeOutput)
	}

	tx.Body.Outputs = append(tx.Body.Outputs, changeOutput)
	return nil
}

func DebugAmounts(
	inputAmounts map[xc.ContractAddress]xc.AmountBlockchain,
	outputAmounts map[xc.ContractAddress]xc.AmountBlockchain,
	changeOutput *Output,
) {
	input := log.Fields{}
	for contract, amount := range inputAmounts {
		input[string(contract)] = amount.String()
	}
	output := log.Fields{}
	for contract, amount := range outputAmounts {
		output[string(contract)] = amount.String()
	}

	change := log.Fields{}
	changeAmounts := changeOutput.TokenAmounts
	change[types.Lovelace] = changeAmounts.NativeAmount
	policyIter := changeAmounts.PolicyIdToAmounts.Iter()
	for ok := policyIter.First(); ok; ok = policyIter.Next() {
		policyId := policyIter.Key()
		assetAmounts := policyIter.Value()
		assetIter := assetAmounts.Iter()
		for ok := assetIter.First(); ok; ok = assetIter.Next() {
			assetName := assetIter.Key()
			assetAmount := assetIter.Value()
			change[string(policyId)+string(assetName)] = assetAmount
		}
	}

	log.WithFields(input).Debug("input amounts")
	log.WithFields(output).Debug("output amounts")
	log.WithFields(change).Debug("change amounts, before fee")
}

func (tx *Tx) SetMemo(memo string) error {
	tx.Memo = &memo
	bytes, err := cbor.Marshal(memo)
	if err != nil {
		return errors.New("failed to marshal memo")
	}
	hash := blake2b.Sum256(bytes)
	tx.Body.MemoHash = hash[:]
	return nil
}

func (tx *Tx) SetTTL(ttl uint32) {
	if ttl > 0 {
		tx.Body.TTL = ttl
	}
}

func (tx *Tx) SetUtxos(utxos []types.Utxo) error {
	for _, utxo := range utxos {
		hexHash, err := hex.DecodeString(utxo.TxHash)
		if err != nil {
			return fmt.Errorf("failed to decode hash: %w", err)
		}
		txInput := &Input{
			TxHash: hexHash,
			Index:  utxo.Index,
		}
		tx.Body.Inputs = append(tx.Body.Inputs, txInput)
	}
	return nil
}

func (tx *Tx) UpdateChangeAmount(diff int64) error {
	outLen := len(tx.Body.Outputs)
	if outLen == 0 {
		return errors.New("cannot set fee without a change output")
	}

	if diff < 0 {
		absDiff := uint64(-diff)
		if absDiff > tx.Body.Outputs[outLen-1].TokenAmounts.NativeAmount {
			return errors.New("output will be left with negative amount")
		}

		tx.Body.Outputs[outLen-1].TokenAmounts.NativeAmount -= absDiff
	} else {

		tx.Body.Outputs[outLen-1].TokenAmounts.NativeAmount += uint64(diff)
	}

	return nil
}

// Set fee and deduce the amount from last output
func (tx *Tx) SetFee(fee uint64) error {
	outLen := len(tx.Body.Outputs)
	if outLen == 0 {
		return errors.New("cannot set fee without a change output")
	}

	tx.Body.Fee = fee
	tx.UpdateChangeAmount(int64(-fee))
	return nil
}

// Set certificates and deduce deposit from last output
func (tx *Tx) SetCertificates(certs []Certificate) error {
	outLen := len(tx.Body.Outputs)
	if outLen == 0 {
		return errors.New("cannot set certificate without a change output")
	}

	tx.Body.Certificates = certs
	for _, c := range certs {
		var changeDiff int64
		// Increase change output if we are deregistering and decrese in case of registering
		if c.CertificationType == CertTypeDeregistration {
			changeDiff = int64(c.DepositAmount)
		} else if c.CertificationType == CertTypeRegistrationAndDelegation {
			changeDiff = int64(-c.DepositAmount)
		}
		tx.UpdateChangeAmount(changeDiff)
	}

	tx.RequiresAdditionalSignature = true
	return nil
}

// Set withdrawal address and amount, and deduce the amount from last output
func (tx *Tx) SetWithdrawal(rewardsAddress xc.Address, amount xc.AmountBlockchain) error {
	outLen := len(tx.Body.Outputs)
	if outLen == 0 {
		return errors.New("cannot set withdrawal without a change output")
	}

	_, addressBytes, err := DecodeToBase256(string(rewardsAddress))
	if err != nil {
		return fmt.Errorf("failed to decode rewards address: %w", err)
	}

	tx.Body.Withdrawal = &Withdrawal{
		StakeAddress: addressBytes,
		Amount:       amount.Uint64(),
	}

	// Add to change amount - we are withdrawing from stakeRewards address
	tx.UpdateChangeAmount(int64(amount.Uint64()))

	tx.RequiresAdditionalSignature = true
	return nil
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

	sighashes := []*xc.SignatureRequest{signatureData}
	if tx.RequiresAdditionalSignature {
		sighashes = append(sighashes, signatureData)
	}

	return sighashes, nil
}

// SetSignatures adds a signature to Tx
func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	if len(signatures) == 0 {
		return errors.New("no signatures provided")
	}

	if len(tx.Witness.Keys) != 0 {
		return errors.New("tx already signed")
	}

	sighashes, err := tx.Sighashes()
	if err != nil {
		return errors.New("failed to get sighashes")
	}
	if len(sighashes) != len(signatures) {
		return fmt.Errorf(
			"invalid signature count, expected: %d, got: %d",
			len(sighashes), len(signatures),
		)
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

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	txCbor, err := cbor.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tx: %w", err)
	}

	return txCbor, nil
}

func NewTransfer(args builder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	txInput, ok := input.(*tx_input.TxInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	tx := NewTx()
	inputMemo, ok := args.GetMemo()
	if ok {
		tx.SetMemo(inputMemo)
	}

	tx.SetUtxos(txInput.Utxos)

	// Recipient output
	to := args.GetTo()
	amount := args.GetAmount()
	contract, _ := args.GetContract()
	err := tx.CreateTransferOutput(to, amount, contract)
	if err != nil {
		return nil, fmt.Errorf("failed to create transfer output: %w", err)
	}

	// Change output
	err = tx.CreateChangeOutput(txInput.Utxos, args.GetFrom())
	if err != nil {
		return nil, fmt.Errorf("failed to create change output: %w", err)
	}

	tx.SetTTL(uint32(txInput.Slot + txInput.TransactionValidityTime))

	err = tx.SetFee(txInput.Fee)
	if err != nil {
		return nil, fmt.Errorf("failed to set tx fee: %w", err)
	}
	return tx, nil
}

func NewStake(args builder.StakeArgs, input xc.StakeTxInput) (xc.Tx, error) {
	_, ok := args.GetAmount()
	if ok {
		return nil, buildererrors.ErrStakingAmountNotUsed
	}
	stakingInput, ok := input.(*tx_input.StakingInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}
	txInput := stakingInput.TxInput

	transaction := NewTx()
	transaction.SetUtxos(txInput.Utxos)

	// Change output
	err := transaction.CreateChangeOutput(txInput.Utxos, args.GetFrom())
	if err != nil {
		return nil, fmt.Errorf("failed to create change output: %w", err)
	}

	pubkey, ok := args.GetPublicKey()
	if !ok {
		return nil, fmt.Errorf("cardano staking requires public key arg")
	}

	credential, err := NewKeyCredential(pubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to create key credential: %w", err)
	}
	poolId, ok := args.GetValidator()
	if !ok {
		return nil, fmt.Errorf("pool id is required for cardano staking, use '--validator'")
	}
	poolBytes, err := hex.DecodeString(poolId)
	if err != nil {
		return nil, fmt.Errorf("failed to decode pool id: %w", err)
	}
	transaction.SetCertificates([]Certificate{
		{
			CertificationType: CertTypeRegistrationAndDelegation,
			Credential:        credential,
			PoolId:            poolBytes,
			DepositAmount:     stakingInput.KeyDeposit,
		},
	})

	transaction.SetTTL(uint32(txInput.Slot + txInput.TransactionValidityTime))

	err = transaction.SetFee(txInput.Fee)
	if err != nil {
		return nil, fmt.Errorf("failed to set tx fee: %w", err)
	}

	return transaction, nil
}

func NewUnstake(args builder.StakeArgs, input xc.UnstakeTxInput) (xc.Tx, error) {
	_, ok := args.GetAmount()
	if ok {
		return nil, buildererrors.ErrStakingAmountNotUsed
	}
	unstakingInput, ok := input.(*tx_input.UnstakingInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}
	txInput := unstakingInput.TxInput

	transaction := NewTx()
	transaction.SetUtxos(txInput.Utxos)

	err := transaction.CreateChangeOutput(txInput.Utxos, args.GetFrom())
	if err != nil {
		return nil, fmt.Errorf("failed to create change output: %w", err)
	}

	pubkey, ok := args.GetPublicKey()
	if !ok {
		return nil, fmt.Errorf("cardano staking requires public key arg")
	}

	credential, err := NewKeyCredential(pubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to create key credential: %w", err)
	}

	transaction.SetCertificates([]Certificate{
		{
			CertificationType: CertTypeDeregistration,
			Credential:        credential,
			DepositAmount:     unstakingInput.KeyDeposit,
		},
	})

	transaction.SetTTL(uint32(txInput.Slot + txInput.TransactionValidityTime))

	err = transaction.SetFee(txInput.Fee)
	if err != nil {
		return nil, fmt.Errorf("failed to set tx fee: %w", err)
	}

	return transaction, nil
}

func NewWithdraw(args builder.StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error) {
	_, ok := args.GetAmount()
	if ok {
		return nil, buildererrors.ErrStakingAmountNotUsed
	}
	withdrawInput, ok := input.(*tx_input.WithdrawInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}
	txInput := withdrawInput.TxInput

	transaction := NewTx()
	transaction.SetUtxos(txInput.Utxos)

	err := transaction.CreateChangeOutput(txInput.Utxos, args.GetFrom())
	if err != nil {
		return nil, fmt.Errorf("failed to create change output: %w", err)
	}

	transaction.SetTTL(uint32(txInput.Slot + txInput.TransactionValidityTime))

	err = transaction.SetFee(txInput.Fee)
	if err != nil {
		return nil, fmt.Errorf("failed to set tx fee: %w", err)
	}

	err = transaction.SetWithdrawal(withdrawInput.RewardsAddress, withdrawInput.RewardsAmount)
	if err != nil {
		return nil, fmt.Errorf("failed to set withdrawal: %w", err)
	}

	return transaction, nil
}
