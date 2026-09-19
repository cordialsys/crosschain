package tempo

import (
	"fmt"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	evmtx "github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

const tempoTransactionType byte = 0x76
const tempoFeePayerDomain byte = 0x78

// Tx wraps the call fields of an EIP-1559 transaction in a Tempo envelope.
// Native sponsorship uses a separate signature and consumes only the sender nonce.
type Tx struct {
	envelope          tempoEnvelope
	sender            xc.Address
	feePayer          xc.Address
	signature         []byte
	feePayerSignature []byte
}

var _ xc.Tx = &Tx{}
var _ xc.TxAdditionalSighashes = &Tx{}

// NewTx snapshots an EVM transaction into an unsigned, sender-paid Tempo envelope.
func NewTx(evmTx *evmtx.Tx, feeContract xc.ContractAddress) (*Tx, error) {
	if evmTx == nil {
		return nil, fmt.Errorf("EVM transaction is nil")
	}
	ethTx, err := evmTx.BuildEthTx()
	if err != nil {
		return nil, fmt.Errorf("building EVM transaction: %w", err)
	}
	return newTempoTx(ethTx, feeContract)
}

func newTempoTx(ethTx *types.Transaction, feeContract xc.ContractAddress) (*Tx, error) {
	if ethTx.Type() != types.DynamicFeeTxType {
		return nil, fmt.Errorf("unsupported EVM transaction type %d for Tempo", ethTx.Type())
	}
	if ethTx.ChainId().Sign() <= 0 || !ethTx.ChainId().IsUint64() {
		return nil, fmt.Errorf("Tempo chain ID must be a positive uint64")
	}
	if ethTx.Gas() == 0 {
		return nil, fmt.Errorf("Tempo gas limit must be positive")
	}
	tip, fee := ethTx.GasTipCap(), ethTx.GasFeeCap()
	if tip.Sign() < 0 || fee.Sign() < 0 || tip.BitLen() > 128 || fee.BitLen() > 128 || tip.Cmp(fee) > 0 {
		return nil, fmt.Errorf("Tempo gas caps must fit uint128 and priority fee cannot exceed max fee")
	}
	if feeContract != "" && (!common.IsHexAddress(string(feeContract)) || common.HexToAddress(string(feeContract)) == (common.Address{})) {
		return nil, fmt.Errorf("invalid Tempo fee contract %q", feeContract)
	}
	return &Tx{envelope: tempoEnvelope{
		ChainID:   ethTx.ChainId(),
		GasTipCap: tip,
		GasFeeCap: fee,
		Gas:       ethTx.Gas(),
		Calls: []tempoCall{{
			To:    ethTx.To(),
			Value: ethTx.Value(),
			Data:  ethTx.Data(),
		}},
		AccessList:        ethTx.AccessList(),
		Nonce:             ethTx.Nonce(),
		FeeToken:          feeContractBytes(feeContract),
		FeePayer:          []byte{},
		AuthorizationList: []any{},
	}}, nil
}

func (tx Tx) Hash() xc.TxHash {
	if len(tx.signature) == 0 {
		return ""
	}
	serialized, err := tx.Serialize()
	if err != nil {
		return ""
	}
	return xc.TxHash(crypto.Keccak256Hash(serialized).Hex())
}

func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	envelope := tx.envelope
	if tx.feePayer != "" {
		// The sender delegates fee-token choice to the sponsor.
		envelope.FeeToken, envelope.FeePayer = nil, []byte{0}
	}
	serialized, err := envelope.encode(tempoTransactionType)
	if err != nil {
		return nil, err
	}
	return []*xc.SignatureRequest{xc.NewSignatureRequest(crypto.Keccak256(serialized), tx.sender)}, nil
}

func (tx Tx) feePayerSighash() ([]byte, error) {
	envelope := tx.envelope
	envelope.FeePayer = common.HexToAddress(string(tx.sender))
	serialized, err := envelope.encode(tempoFeePayerDomain)
	if err != nil {
		return nil, err
	}
	return crypto.Keccak256(serialized), nil
}

func (tx Tx) AdditionalSighashes() ([]*xc.SignatureRequest, error) {
	if tx.feePayer == "" || len(tx.feePayerSignature) != 0 {
		return nil, nil
	}
	if len(tx.signature) == 0 {
		return nil, fmt.Errorf("missing sender signature")
	}
	hash, err := tx.feePayerSighash()
	if err != nil {
		return nil, err
	}
	return []*xc.SignatureRequest{xc.NewSignatureRequest(hash, tx.feePayer)}, nil
}

func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
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
		hash, err := tx.feePayerSighash()
		if err != nil {
			return err
		}
		if err := verifyTempoSignature(signatures[1], hash, tx.feePayer); err != nil {
			return err
		}
	}
	tx.signature = append([]byte(nil), signatures[0].Signature...)
	// Geth signers return 0/1; Tempo's canonical sender envelope uses 27/28.
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
	// The low-level NewTx constructor has no sender argument. Builders always set it.
	if signer != "" && crypto.PubkeyToAddress(*key) != common.HexToAddress(string(signer)) {
		return fmt.Errorf("Tempo signature does not match expected signer %s", signer)
	}
	return nil
}

func (tx Tx) Serialize() ([]byte, error) {
	envelope := tx.envelope
	envelope.Signature = tx.signature
	if tx.feePayer != "" {
		if len(tx.signature) == 0 || len(tx.feePayerSignature) == 0 {
			return nil, fmt.Errorf("Tempo sponsored transaction requires sender and fee-payer signatures")
		}
		sig := tx.feePayerSignature
		envelope.FeePayer = []any{uint64(sig[64]), new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:64])}
	}
	return envelope.encode(tempoTransactionType)
}

type tempoCall struct {
	To    *common.Address
	Value *big.Int
	Data  []byte
}

// Field order is defined by Tempo's transaction RLP format. FeePayer is an empty
// string, a signing marker, a sender address, or an RLP signature tuple.
type tempoEnvelope struct {
	ChainID           *big.Int
	GasTipCap         *big.Int
	GasFeeCap         *big.Int
	Gas               uint64
	Calls             []tempoCall
	AccessList        types.AccessList
	NonceKey          uint64
	Nonce             uint64
	ValidBefore       []byte
	ValidAfter        []byte
	FeeToken          []byte
	FeePayer          any
	AuthorizationList []any
	Signature         []byte `rlp:"optional"`
}

func (envelope tempoEnvelope) encode(domain byte) ([]byte, error) {
	if envelope.ChainID == nil {
		return nil, fmt.Errorf("Tempo transaction not initialized")
	}
	payload, err := rlp.EncodeToBytes(envelope)
	if err != nil {
		return nil, fmt.Errorf("encoding Tempo transaction: %w", err)
	}
	return append([]byte{domain}, payload...), nil
}

func feeContractBytes(contract xc.ContractAddress) []byte {
	if contract == "" {
		return nil
	}
	return common.HexToAddress(string(contract)).Bytes()
}

var NewMultiTx = evmtx.NewMultiTx
