package tx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cosmos/address"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cometbft/cometbft/crypto/tmhash"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
)

// Tx for Cosmos
type Tx struct {
	ChainCfg        *xc.ChainBaseConfig
	Input           tx_input.TxInput
	Msgs            []types.Msg
	Fees            types.Coins
	SignerPublicKey []byte
	Memo            string

	signatures [][]byte
}

func NewTx(chain *xc.ChainBaseConfig, input tx_input.TxInput, msgs []types.Msg, fees types.Coins, senderPubkey []byte, memo string) *Tx {
	signatures := [][]byte{}
	return &Tx{
		chain, input, msgs, fees, senderPubkey, memo, signatures,
	}
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
	signDoc, err := tx.BuildUnsigned()
	if err != nil {
		return nil, err
	}
	signDocBytes, err := signDoc.Marshal()
	if err != nil {
		return nil, err
	}

	sighash := GetSighash(tx.ChainCfg, signDocBytes)
	return []xc.TxDataToSign{sighash}, nil
}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	if len(signatures) == 0 {
		return fmt.Errorf("invalid signatures size")
	}
	for _, signature := range signatures {
		sig := signature[:]
		if len(sig) > 64 {
			sig = sig[:64]
		}
		if len(signature) == 0 {
			return fmt.Errorf("invalid signature size")
		}
		tx.signatures = append(tx.signatures, sig)
	}
	return nil
}

func (tx Tx) GetSignatures() []xc.TxSignature {
	sigs := []xc.TxSignature{}
	for _, sig := range tx.signatures {
		sigs = append(sigs, sig)
	}

	return sigs
}

// Serialize serializes a Tx
func (tx Tx) Serialize() ([]byte, error) {
	signDoc, err := tx.BuildUnsigned()
	if err != nil {
		return nil, err
	}

	txRaw := &sdktx.TxRaw{
		BodyBytes:     signDoc.BodyBytes,
		AuthInfoBytes: signDoc.AuthInfoBytes,
		Signatures:    tx.signatures,
	}
	serialized, err := txRaw.Marshal()
	if err != nil {
		return nil, err
	}
	return serialized, nil
}

func GetSighash(asset *xc.ChainBaseConfig, sigData []byte) []byte {
	if address.IsEVMOS(asset) {
		return crypto.Keccak256(sigData)
	}
	sighash := sha256.Sum256(sigData)
	return sighash[:]
}

func (tx Tx) BuildUnsigned() (*sdktx.SignDoc, error) {
	body := &sdktx.TxBody{
		Memo:          tx.Memo,
		TimeoutHeight: tx.Input.TimeoutHeight,
	}
	msgsAny, err := sdktx.SetMsgs(tx.Msgs)
	if err != nil {
		return nil, err
	}
	body.Messages = msgsAny

	pubkey := address.GetPublicKey(tx.ChainCfg, tx.SignerPublicKey)
	pubkeyAny, err := codectypes.NewAnyWithValue(pubkey)
	if err != nil {
		return nil, err
	}

	mode := sdktx.ModeInfo_Single_{
		Single: &sdktx.ModeInfo_Single{
			Mode: signingtypes.SignMode_SIGN_MODE_DIRECT,
		},
	}
	modeInfo := &sdktx.ModeInfo{
		Sum: &mode,
	}

	signerInfo := []*sdktx.SignerInfo{
		{PublicKey: pubkeyAny, ModeInfo: modeInfo, Sequence: tx.Input.Sequence},
	}
	fee := &sdktx.Fee{Amount: tx.Fees, GasLimit: tx.Input.GasLimit}
	authInfo := sdktx.AuthInfo{SignerInfos: signerInfo, Fee: fee}

	bodyBytes, err := body.Marshal()
	if err != nil {
		return nil, err
	}
	authInfoBytes, err := authInfo.Marshal()
	if err != nil {
		return nil, err
	}

	chainId := tx.Input.ChainId
	if chainId == "" {
		chainId = tx.ChainCfg.ChainID.AsString()
	}
	all := sdktx.SignDoc{
		BodyBytes:     bodyBytes,
		AuthInfoBytes: authInfoBytes,
		ChainId:       chainId,
		AccountNumber: tx.Input.AccountNumber,
	}

	return &all, nil
}
