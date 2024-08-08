package tx

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cosmos/address"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cometbft/cometbft/crypto/tmhash"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
)

// Tx for Cosmos
type Tx struct {
	CosmosTx        types.Tx
	ParsedTransfers []types.Msg
	// aux fields
	CosmosTxBuilder client.TxBuilder
	CosmosTxEncoder types.TxEncoder
	SigsV2          []signingtypes.SignatureV2
	InputSignatures []xc.TxSignature
	TxDataToSign    []byte
}

var _ xc.Tx = &Tx{}

type Cw20MsgTransfer struct {
	Transfer *Cw20Transfer `json:"transfer,omitempty"`
}
type Cw20Transfer struct {
	Amount    string `json:"amount,omitempty"`
	Recipient string `json:"recipient,omitempty"`
}

func TmHash(bz []byte) xc.TxHash {
	txID := tmhash.Sum(bz)
	return xc.TxHash(hex.EncodeToString(txID))
}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	serialized, err := tx.Serialize()
	if err != nil || serialized == nil || len(serialized) == 0 {
		return ""
	}
	return TmHash(serialized)
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]xc.TxDataToSign, error) {
	if tx.TxDataToSign == nil {
		return nil, errors.New("transaction not initialized")
	}
	return []xc.TxDataToSign{tx.TxDataToSign}, nil
}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	if tx.SigsV2 == nil || len(tx.SigsV2) < 1 || tx.CosmosTxBuilder == nil {
		return errors.New("transaction not initialized")
	}
	if len(signatures) != len(tx.SigsV2) {
		return errors.New("invalid signatures size")
	}
	for i, signature := range signatures {
		sig := signature[:]
		if len(sig) > 64 {
			sig = sig[:64]
		}
		data := tx.SigsV2[i].Data
		signMode := data.(*signingtypes.SingleSignatureData).SignMode
		tx.SigsV2[i].Data = &signingtypes.SingleSignatureData{
			SignMode:  signMode,
			Signature: sig,
		}
	}
	tx.InputSignatures = signatures
	return tx.CosmosTxBuilder.SetSignatures(tx.SigsV2...)
}

func (tx Tx) GetSignatures() []xc.TxSignature {
	return tx.InputSignatures
}

// Serialize serializes a Tx
func (tx Tx) Serialize() ([]byte, error) {
	if tx.CosmosTxEncoder == nil {
		return []byte{}, errors.New("transaction not initialized")
	}

	// if CosmosTxBuilder is set, prioritize GetTx()
	txToEncode := tx.CosmosTx
	if tx.CosmosTxBuilder != nil {
		txToEncode = tx.CosmosTxBuilder.GetTx()
	}

	if txToEncode == nil {
		return []byte{}, errors.New("transaction not initialized")
	}
	serialized, err := tx.CosmosTxEncoder(txToEncode)
	return serialized, err
}

// Fee returns the fee of a Tx
func (tx Tx) Fee() xc.AmountBlockchain {
	switch tf := tx.CosmosTx.(type) {
	case types.FeeTx:
		fee := tf.GetFee()[0].Amount.BigInt()
		return xc.AmountBlockchain(*fee)
	}
	return xc.NewAmountBlockchainFromUint64(0)
}

func GetSighash(asset *xc.ChainConfig, sigData []byte) []byte {
	if address.IsEVMOS(asset) {
		return crypto.Keccak256(sigData)
	}
	sighash := sha256.Sum256(sigData)
	return sighash[:]
}
