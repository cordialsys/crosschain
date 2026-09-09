package tempo

import (
	"errors"
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

// Tx is a Tempo transaction built from an EIP-1559 transaction. Tempo keeps
// the EVM transaction fields, but encodes the destination/value/data as a call
// and adds its fee-token field to the signed transaction envelope.
type Tx struct {
	evmTx       *evmtx.Tx
	feeContract xc.ContractAddress
	signature   xc.TxSignature
}

var _ xc.Tx = &Tx{}

func NewTx(evmTx *evmtx.Tx, feeContract xc.ContractAddress) (*Tx, error) {
	if evmTx == nil {
		return nil, fmt.Errorf("EVM transaction is nil")
	}
	if feeContract != "" && !common.IsHexAddress(string(feeContract)) {
		return nil, fmt.Errorf("invalid Tempo fee contract %q", feeContract)
	}
	return &Tx{evmTx: evmTx, feeContract: feeContract}, nil
}

func (tx Tx) Hash() xc.TxHash {
	serialized, err := tx.Serialize()
	if err != nil {
		return ""
	}
	return xc.TxHash(crypto.Keccak256Hash(serialized).Hex())
}

func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	serialized, err := tx.serialize(nil)
	if err != nil {
		return nil, err
	}
	return []*xc.SignatureRequest{xc.NewSignatureRequest(crypto.Keccak256(serialized))}, nil
}

func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	if len(signatures) != 1 {
		return fmt.Errorf("expected 1 Tempo signature, got %d", len(signatures))
	}
	if len(signatures[0].Signature) != crypto.SignatureLength {
		return fmt.Errorf("invalid Tempo signature length %d", len(signatures[0].Signature))
	}
	tx.signature = append(tx.signature[:0], signatures[0].Signature...)
	return nil
}

func (tx Tx) Serialize() ([]byte, error) {
	return tx.serialize(tx.signature)
}

type tempoCall struct {
	To    *common.Address
	Value *big.Int
	Data  []byte
}

func (tx Tx) serialize(signature []byte) ([]byte, error) {
	ethTx, err := tx.evmTx.BuildEthTx()
	if err != nil {
		return nil, fmt.Errorf("building EVM transaction: %w", err)
	}
	if ethTx.Type() != types.DynamicFeeTxType {
		return nil, fmt.Errorf("unsupported EVM transaction type %d for Tempo", ethTx.Type())
	}
	if ethTx.ChainId() == nil || ethTx.ChainId().Sign() <= 0 {
		return nil, errors.New("tempo chain ID must be positive")
	}
	if ethTx.Gas() == 0 {
		return nil, errors.New("tempo gas limit must be positive")
	}
	if ethTx.GasTipCap().Cmp(ethTx.GasFeeCap()) > 0 {
		return nil, errors.New("tempo gas tip cap exceeds fee cap")
	}

	fields := []interface{}{
		ethTx.ChainId(),
		ethTx.GasTipCap(),
		ethTx.GasFeeCap(),
		ethTx.Gas(),
		[]tempoCall{{To: ethTx.To(), Value: ethTx.Value(), Data: ethTx.Data()}},
		ethTx.AccessList(),
		[]byte{}, // nonce key: use Tempo's default sequential nonce space
		ethTx.Nonce(),
		[]byte{}, // valid before
		[]byte{}, // valid after
		feeContractBytes(tx.feeContract),
		[]byte{},        // no fee payer
		[]interface{}{}, // reserved EIP-7702 authorization list
	}
	if len(signature) != 0 {
		fields = append(fields, signature)
	}

	payload, err := rlp.EncodeToBytes(fields)
	if err != nil {
		return nil, fmt.Errorf("encoding Tempo transaction: %w", err)
	}
	return append([]byte{tempoTransactionType}, payload...), nil
}

func feeContractBytes(contract xc.ContractAddress) []byte {
	if contract == "" {
		return []byte{}
	}
	return common.HexToAddress(string(contract)).Bytes()
}

var NewMultiTx = evmtx.NewMultiTx
