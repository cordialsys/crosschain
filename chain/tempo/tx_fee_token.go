package tempo

import (
	"fmt"
	"math/big"
	"strings"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	evmtx "github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

const feeTokenTxType byte = 0x76

// FeeTokenTx is the minimal Tempo envelope: one TIP-20 transfer, sequential
// nonce (nonceKey=0), an optional native fee payer, and secp256k1 signatures.
// It implements xc.Tx directly because geth's types.Transaction cannot encode
// Tempo's transaction type. See the reference field order and signing rules:
// https://github.com/tempoxyz/tempo/blob/main/crates/primitives/src/transaction/tempo_transaction.rs
type FeeTokenTx struct {
	fields            []any
	signature         []byte
	sender            xc.Address
	feePayer          xc.Address
	feePayerSignature []byte
}

var _ xc.Tx = &FeeTokenTx{}
var _ xc.TxAdditionalSighashes = &FeeTokenTx{}

func validateFeeTokenTransfer(args xcbuilder.TransferArgs) error {
	if payer, ok := args.GetFeePayer(); ok {
		if !common.IsHexAddress(string(payer)) || common.HexToAddress(string(payer)) == (common.Address{}) {
			return fmt.Errorf("Tempo requires a valid fee-payer address")
		}
		if strings.EqualFold(string(payer), string(args.GetFrom())) {
			return fmt.Errorf("Tempo native fee payer must differ from the sender")
		}
	}
	contract, ok := args.GetContract()
	if !ok || !common.IsHexAddress(string(contract)) {
		return fmt.Errorf("Tempo requires a valid transfer token contract")
	}
	feeContract, ok := args.GetFeeContract()
	if !ok || !common.IsHexAddress(string(feeContract)) || common.HexToAddress(string(feeContract)) == (common.Address{}) {
		return fmt.Errorf("Tempo requires a valid fee token contract")
	}
	if !common.IsHexAddress(string(args.GetFrom())) || !common.IsHexAddress(string(args.GetTo())) {
		return fmt.Errorf("Tempo requires valid sender and recipient addresses")
	}
	if args.GetAmount().Int().Sign() < 0 || args.GetAmount().Int().BitLen() > 256 {
		return fmt.Errorf("Tempo transfer amount must fit uint256")
	}
	return nil
}

func newFeeTokenTx(chain *xc.ChainBaseConfig, args xcbuilder.TransferArgs, input *TxInput) (*FeeTokenTx, error) {
	if err := validateFeeTokenTransfer(args); err != nil {
		return nil, err
	}
	feePayer, sponsored := args.GetFeePayer()
	if input.NativeFeePayer != sponsored || !strings.EqualFold(string(input.FeePayerAddress), string(feePayer)) || input.FeePayerNonce != 0 {
		return nil, fmt.Errorf("Tempo fee payer differs from native sponsorship input")
	}
	chainID := evmtx.GetChainId(chain, &input.TxInput)
	if chainID.IsZero() || !chainID.IsUint64() {
		return nil, fmt.Errorf("Tempo chain ID must be a positive uint64")
	}
	tip, fee := input.GasTipCap.Int(), input.GasFeeCap.Int()
	if tip.Sign() < 0 || fee.Sign() < 0 || tip.BitLen() > 128 || fee.BitLen() > 128 || tip.Cmp(fee) > 0 {
		return nil, fmt.Errorf("Tempo gas caps must fit uint128 and priority fee cannot exceed max fee")
	}
	to, value, data, err := evmtx.EvmDestinationAndAmountAndData(args.GetTo(), args.GetAmount(), &args)
	if err != nil {
		return nil, err
	}
	feeContract, _ := args.GetFeeContract()
	return &FeeTokenTx{fields: []any{
		chainID.ToBig(), new(big.Int).Set(tip), new(big.Int).Set(fee), input.GasLimit,
		[]any{[]any{to, value, data}}, // calls: [[to, value, input]]
		[]any{},                       // accessList
		uint64(0), input.Nonce,        // nonceKey, nonce
		[]byte{}, []byte{}, // validBefore, validAfter
		common.HexToAddress(string(feeContract)),
		[]byte{}, // no feePayerSignature: sender commits to feeToken
		[]any{},  // authorizationList; absent keyAuthorization adds no field
	}, sender: args.GetFrom(), feePayer: feePayer}, nil
}

func (tx *FeeTokenTx) encode(signed bool) ([]byte, error) {
	if tx == nil || len(tx.fields) == 0 {
		return nil, fmt.Errorf("transaction not initialized")
	}
	fields := append([]any(nil), tx.fields...)
	if tx.feePayer != "" {
		// The sender authorizes sponsorship without committing to its currency.
		// Only the sponsor signs feeToken, with domain byte 0x78.
		fields[10], fields[11] = []byte{}, []byte{0}
	}
	if signed {
		if len(tx.signature) != crypto.SignatureLength {
			return nil, fmt.Errorf("Tempo transaction requires a sender signature")
		}
		if tx.feePayer != "" {
			if len(tx.feePayerSignature) != crypto.SignatureLength {
				return nil, fmt.Errorf("Tempo transaction requires a fee-payer signature")
			}
			sig := tx.feePayerSignature
			fields[10] = tx.fields[10]
			fields[11] = []any{uint64(sig[64]), new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:64])}
		}
		fields = append(fields, tx.signature)
	}
	payload, err := rlp.EncodeToBytes(fields)
	if err != nil {
		return nil, err
	}
	return append([]byte{feeTokenTxType}, payload...), nil
}

func (tx *FeeTokenTx) Sighashes() ([]*xc.SignatureRequest, error) {
	payload, err := tx.encode(false)
	if err != nil {
		return nil, err
	}
	return []*xc.SignatureRequest{xc.NewSignatureRequest(crypto.Keccak256(payload), tx.sender)}, nil
}

func (tx *FeeTokenTx) AdditionalSighashes() ([]*xc.SignatureRequest, error) {
	if tx.feePayer == "" || len(tx.feePayerSignature) > 0 {
		return nil, nil
	}
	if len(tx.signature) == 0 {
		return nil, fmt.Errorf("missing sender signature")
	}
	payload, err := tx.feePayerSighash()
	if err != nil {
		return nil, err
	}
	return []*xc.SignatureRequest{xc.NewSignatureRequest(payload, tx.feePayer)}, nil
}

func (tx *FeeTokenTx) feePayerSighash() ([]byte, error) {
	fields := append([]any(nil), tx.fields...)
	fields[11] = common.HexToAddress(string(tx.sender))
	payload, err := rlp.EncodeToBytes(fields)
	if err != nil {
		return nil, err
	}
	return crypto.Keccak256(append([]byte{0x78}, payload...)), nil
}

func (tx *FeeTokenTx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	if len(signatures) < 1 || len(signatures) > 2 || (tx.feePayer == "" && len(signatures) != 1) {
		return fmt.Errorf("Tempo expects sender signature followed by optional fee-payer signature")
	}
	requests, err := tx.Sighashes()
	if err != nil {
		return err
	}
	if err := verifyTempoSignature(signatures[0], requests[0].Payload, tx.sender); err != nil {
		return err
	}
	if len(signatures) == 2 {
		payload, err := tx.feePayerSighash()
		if err != nil {
			return err
		}
		if err := verifyTempoSignature(signatures[1], payload, tx.feePayer); err != nil {
			return err
		}
	}
	tx.signature = append([]byte(nil), signatures[0].Signature...)
	// Tempo's signature envelope uses Ethereum's 27/28 recovery byte; the
	// crosschain/geth signer returns 0/1. No signature-type prefix is added.
	tx.signature[64] += 27
	tx.feePayerSignature = nil
	if len(signatures) == 2 {
		tx.feePayerSignature = append([]byte(nil), signatures[1].Signature...)
	}
	return nil
}

func verifyTempoSignature(response *xc.SignatureResponse, payload []byte, signer xc.Address) error {
	if response == nil || len(response.Signature) != crypto.SignatureLength {
		return fmt.Errorf("Tempo requires a 65-byte secp256k1 signature")
	}
	sig := response.Signature
	if !crypto.ValidateSignatureValues(sig[64], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:64]), true) {
		return fmt.Errorf("invalid Tempo secp256k1 signature")
	}
	key, err := crypto.SigToPub(payload, sig)
	if err != nil {
		return err
	}
	if crypto.PubkeyToAddress(*key) != common.HexToAddress(string(signer)) {
		return fmt.Errorf("Tempo signature does not match expected signer %s", signer)
	}
	return nil
}

func (tx *FeeTokenTx) Serialize() ([]byte, error) {
	return tx.encode(true)
}

func (tx *FeeTokenTx) Hash() xc.TxHash {
	payload, err := tx.Serialize()
	if err != nil {
		return ""
	}
	return xc.TxHash(crypto.Keccak256Hash(payload).Hex())
}
