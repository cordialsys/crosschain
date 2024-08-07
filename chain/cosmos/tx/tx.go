package tx

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cosmos/address"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cometbft/cometbft/crypto/tmhash"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
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

// ParseTransfer parses a Tx as a transfer
// Currently only banktypes.MsgSend is implemented, i.e. only native tokens
func (tx *Tx) ParseTransfer() {
	for _, msg := range tx.CosmosTx.GetMsgs() {
		switch msg := msg.(type) {
		case *banktypes.MsgSend:
			tx.ParsedTransfers = append(tx.ParsedTransfers, msg)
		case *wasmtypes.MsgExecuteContract:
			tx.ParsedTransfers = append(tx.ParsedTransfers, msg)
		}
	}
}

// From returns the from address of a Tx
func (tx Tx) From() xc.Address {
	for _, parsedTransfer := range tx.ParsedTransfers {
		switch tf := parsedTransfer.(type) {
		case *banktypes.MsgSend:
			from := tf.FromAddress
			return xc.Address(from)
		case *wasmtypes.MsgExecuteContract:
			return xc.Address(tf.Sender)
		}
	}
	return xc.Address("")
}

// To returns the to address of a Tx
func (tx Tx) To() xc.Address {
	for _, parsedTransfer := range tx.ParsedTransfers {
		switch tf := parsedTransfer.(type) {
		case *banktypes.MsgSend:
			to := tf.ToAddress
			return xc.Address(to)
		case *wasmtypes.MsgExecuteContract:
			msg := Cw20MsgTransfer{}
			_ = json.Unmarshal(tf.Msg, &msg)
			if msg.Transfer != nil {
				return xc.Address(msg.Transfer.Recipient)
			}
		}

	}
	return xc.Address("")
}

// ContractAddress returns the contract address of a Tx, if any
func (tx Tx) ContractAddress() xc.ContractAddress {
	for _, parsedTransfer := range tx.ParsedTransfers {
		switch tf := parsedTransfer.(type) {
		case *banktypes.MsgSend:
			denom := tf.Amount[0].Denom
			// Previously, we used to null out the denom as contract address to be consistent with other chains,
			// but this is inaccurate as the denom is a valid contract address.
			// if len(denom) < LEN_NATIVE_ASSET {
			// 	denom = ""
			// }
			return xc.ContractAddress(denom)
		case *wasmtypes.MsgExecuteContract:
			return xc.ContractAddress(tf.Contract)
		}
	}
	return xc.ContractAddress("")
}

// Amount returns the amount of a Tx
func (tx Tx) Amount() xc.AmountBlockchain {
	for _, parsedTransfer := range tx.ParsedTransfers {
		switch tf := parsedTransfer.(type) {
		case *banktypes.MsgSend:
			amount := tf.Amount[0].Amount.BigInt()
			return xc.AmountBlockchain(*amount)
		case *wasmtypes.MsgExecuteContract:
			msg := Cw20MsgTransfer{}
			_ = json.Unmarshal(tf.Msg, &msg)
			if msg.Transfer != nil {
				return xc.NewAmountBlockchainFromStr(msg.Transfer.Amount)
			}
		}
	}
	return xc.NewAmountBlockchainFromUint64(0)
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

// Sources returns the sources of a Tx
func (tx Tx) Sources() []*xc.LegacyTxInfoEndpoint {
	sources := []*xc.LegacyTxInfoEndpoint{}
	for _, parsedTransfer := range tx.ParsedTransfers {
		switch tf := parsedTransfer.(type) {
		case *banktypes.MsgSend:
			from := tf.FromAddress
			sources = append(sources, &xc.LegacyTxInfoEndpoint{
				Address: xc.Address(from),
			})
			// currently assume/support single-source transfers
			return sources
		case *wasmtypes.MsgExecuteContract:
			msg := Cw20MsgTransfer{}
			_ = json.Unmarshal(tf.Msg, &msg)
			if msg.Transfer != nil {
				sources = append(sources, &xc.LegacyTxInfoEndpoint{
					Address:         xc.Address(tf.Sender),
					ContractAddress: xc.ContractAddress(tf.Contract),
					Amount:          xc.NewAmountBlockchainFromStr(msg.Transfer.Amount),
				})
			}
		default:
			// fmt.Printf("unknown type: %T\n", tf)
		}
	}
	return sources
}

// Destinations returns the destinations of a Tx
func (tx Tx) Destinations() []*xc.LegacyTxInfoEndpoint {
	destinations := []*xc.LegacyTxInfoEndpoint{}
	for _, parsedTransfer := range tx.ParsedTransfers {
		switch tf := parsedTransfer.(type) {
		case *banktypes.MsgSend:
			to := tf.ToAddress
			denom := tf.Amount[0].Denom
			amount := tf.Amount[0].Amount.BigInt()
			destinations = append(destinations, &xc.LegacyTxInfoEndpoint{
				Address:         xc.Address(to),
				ContractAddress: xc.ContractAddress(denom),
				Amount:          xc.AmountBlockchain(*amount),
			})
		case *wasmtypes.MsgExecuteContract:
			msg := Cw20MsgTransfer{}
			_ = json.Unmarshal(tf.Msg, &msg)
			if msg.Transfer != nil {
				destinations = append(destinations, &xc.LegacyTxInfoEndpoint{
					Address:         xc.Address(msg.Transfer.Recipient),
					ContractAddress: xc.ContractAddress(tf.Contract),
					Amount:          xc.NewAmountBlockchainFromStr(msg.Transfer.Amount),
				})
			}
		default:
			// fmt.Printf("unknown type: %T\n", tf)
		}
	}
	return destinations
}

func GetSighash(asset *xc.ChainConfig, sigData []byte) []byte {
	if address.IsEVMOS(asset) {
		return crypto.Keccak256(sigData)
	}
	sighash := sha256.Sum256(sigData)
	return sighash[:]
}
