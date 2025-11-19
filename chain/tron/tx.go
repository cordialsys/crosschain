package tron

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/tron/core"
	"github.com/golang/protobuf/proto"
)

// Tx for Tron
//
// Tron Stake and Unstake operations require two transactions
type Tx struct {
	TronTxs  []*core.Transaction
	metadata []*TxMetadata
}

var _ xc.Tx = &Tx{}
var _ xc.TxWithMetadata = &Tx{}

func NewTx(tronTransactions []*core.Transaction) (*Tx, error) {
	if len(tronTransactions) == 0 {
		return nil, errors.New("cannot build transactions with no operations")
	}

	tx := &Tx{
		TronTxs:  make([]*core.Transaction, 0),
		metadata: make([]*TxMetadata, 0),
	}

	for _, ttx := range tronTransactions {
		tx.appendTx(ttx)
	}
	return tx, nil
}

func (tx *Tx) appendTx(ttx *core.Transaction) {
	txHash := TronTxHash(ttx)
	tx.metadata = append(tx.metadata, &TxMetadata{
		Hash:   string(txHash),
		Length: 0, // length is affected by the signature
	})
	tx.TronTxs = append(tx.TronTxs, ttx)
}

func TronTxHash(ttx *core.Transaction) xc.TxHash {
	hashBase, _ := proto.Marshal(ttx.RawData)
	digest := sha256.Sum256(hashBase)
	hash := hex.EncodeToString(digest[:])
	return xc.TxHash(hash)
}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	metlen := len(tx.metadata)
	if metlen == 0 {
		return xc.TxHash("")
	}
	return xc.TxHash(tx.metadata[metlen-1].Hash)
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	if len(tx.TronTxs) == 0 {
		return nil, errors.New("invalid transaction")
	}

	sighashes := make([]*xc.SignatureRequest, len(tx.TronTxs))
	for i, ttx := range tx.TronTxs {
		rawData, err := proto.Marshal(ttx.GetRawData())
		if err != nil {
			return nil, errors.New("unable to get raw data")
		}

		hasher := sha256.New()
		hasher.Write(rawData)
		sighashes[i] = xc.NewSignatureRequest(hasher.Sum(nil))
	}

	return sighashes, nil

}

// SetSignatures adds a signature to Tx
func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	if len(signatures) != len(tx.TronTxs) && len(tx.TronTxs) == len(tx.metadata) {
		return fmt.Errorf("invalid signature count, expected: %d, got: %d", len(tx.TronTxs), len(signatures))
	}

	for i, sig := range signatures {
		tx.TronTxs[i].Signature = append(tx.TronTxs[i].Signature, sig.Signature)
		bz, err := SerializeTronTx(tx.TronTxs[i])
		if err != nil {
			return err
		}
		tx.metadata[i].Length = len(bz)
	}
	return nil
}

func SerializeTronTx(ttx *core.Transaction) ([]byte, error) {
	return proto.Marshal(ttx)
}

// Serialize returns the serialized tx
func (tx *Tx) Serialize() ([]byte, error) {
	if len(tx.TronTxs) == 0 {
		return nil, errors.New("invalid tx")
	}

	transactionsBytes := []byte{}
	for _, ttx := range tx.TronTxs {
		bz, err := proto.Marshal(ttx)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize tx: %w", err)
		}

		transactionsBytes = append(transactionsBytes, bz...)
	}

	return transactionsBytes, nil
}

type TxMetadata struct {
	Hash   string `json:"hash"`
	Length int    `json:"length"`
}

type BroadcastMetadata struct {
	TransactionsData []*TxMetadata `json:"transactions_data"`
}

func (tx *Tx) GetMetadata() ([]byte, bool, error) {
	mtdata := BroadcastMetadata{
		TransactionsData: tx.metadata,
	}

	mtbytes, err := json.Marshal(mtdata)
	if err != nil {
		return nil, false, fmt.Errorf("failed to encode metadata: %w", err)
	}
	return mtbytes, len(mtbytes) > 0, nil
}
