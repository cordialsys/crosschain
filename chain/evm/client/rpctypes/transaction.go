package rpctypes

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Transaction struct {
	BlockHash   string              `json:"blockHash"`
	BlockNumber xc.AmountBlockchain `json:"blockNumber"`
	From        string              `json:"from"`
	Gas         xc.AmountBlockchain `json:"gas"`
	GasPrice    xc.AmountBlockchain `json:"gasPrice"`
	// Or also called "GasTipCap"
	MaxPriorityFeePerGas xc.AmountBlockchain `json:"maxPriorityFeePerGas"`
	// Or also called "GasFeeCap"
	MaxFeePerGas xc.AmountBlockchain `json:"maxFeePerGas"`
	// Or also called "BlobFeeCap"
	MaxFeePerBlobGas xc.AmountBlockchain `json:"maxFeePerBlobGas"`

	BlobHashes []common.Hash `json:"blobVersionedHashes"`

	Hash             string              `json:"hash"`
	Input            string              `json:"input"`
	Nonce            xc.AmountBlockchain `json:"nonce"`
	ToMaybe          *common.Address     `json:"to"`
	TransactionIndex xc.AmountBlockchain `json:"transactionIndex"`
	Value            xc.AmountBlockchain `json:"value"`
	Type             xc.AmountBlockchain `json:"type"`
	AccessList       types.AccessList    `json:"accessList"`
	// ChainID is left as a string because on some really old transactions, it's "" empty string,
	// which is an invalid AmountBlockchain.
	ChainID string              `json:"chainId"`
	V       xc.AmountBlockchain `json:"v"`
	YParity xc.AmountBlockchain `json:"yParity"`
	R       xc.AmountBlockchain `json:"r"`
	S       xc.AmountBlockchain `json:"s"`
}

// type TransactionWrapper struct {
// 	*Transaction
// }

// func (tx *TransactionWrapper) Nonce() uint64 {
// 	return tx.Transaction.Nonce.Uint64()
// }
// func (tx *TransactionWrapper) GasTipCap() *big.Int {
// 	return tx.Transaction.MaxPriorityFeePerGas.Int()
// }
// func (tx *TransactionWrapper) GasFeeCap() *big.Int {
// 	return tx.Transaction.MaxFeePerGas.Int()
// }
// func (tx *TransactionWrapper) GasPrice() *big.Int {
// 	return tx.Transaction.GasPrice.Int()
// }
// func (tx *TransactionWrapper) Gas() uint64 {
// 	return tx.Transaction.Gas.Uint64()
// }
// func (tx *TransactionWrapper) To() *common.Address {
// 	return tx.Transaction.ToMaybe
// }
// func (tx *TransactionWrapper) Value() *big.Int {
// 	return tx.Transaction.V.Int()
// }
// func (tx *TransactionWrapper) Data() []byte {
// 	input := tx.Transaction.Input
// 	input = strings.TrimPrefix("0x", input)
// 	data, _ := hex.DecodeString(input)
// 	return data
// }
// func (tx *TransactionWrapper) AccessList() types.AccessList {
// 	return tx.Transaction.AccessList
// }
// func (tx *TransactionWrapper) BlobGasFeeCap() *big.Int {
// 	return tx.Transaction.MaxFeePerBlobGas.Int()
// }

// func (tx *TransactionWrapper) BlobHashes() []common.Hash {
// 	return tx.Transaction.BlobHashes
// }

// func (tx *Transaction) Signature() []byte {
// 	r := tx.R.Int().Bytes()
// 	s := tx.S.Int().Bytes()
// 	v := tx.V.Int().Bytes()
// 	if len(r) != 32 {
// 		panic("r not 32")
// 	}
// 	if len(s) != 32 {
// 		panic("s not 32")
// 	}
// 	if len(v) != 1 {
// 		panic("v not 32")
// 	}
// 	bz := append(r, s...)
// 	bz = append(bz, v...)
// 	return bz
// }

// func as256(b *big.Int) *uint256.Int {
// 	u := new(uint256.Int)
// 	u.SetFromBig(b)
// 	return u
// }

// func (tx *Transaction) RecoverSender() (xc.Address, bool, error) {
// 	txType := uint8(tx.Type.Uint64())
// 	wtx := TransactionWrapper{tx}
// 	var native *types.Transaction
// 	V := tx.V.Int()
// 	R := tx.R.Int()
// 	S := tx.S.Int()
// 	chainId := tx.ChainID.Int()

// 	switch txType {
// 	case types.LegacyTxType:
// 		native = types.NewTx(&types.LegacyTx{
// 			Nonce:    wtx.Nonce(),
// 			GasPrice: wtx.GasPrice(),
// 			Gas:      wtx.Gas(),
// 			To:       wtx.To(),
// 			Value:    wtx.Value(),
// 			Data:     wtx.Data(),
// 			V:        V,
// 			R:        R,
// 			S:        S,
// 		})
// 	case types.AccessListTxType:
// 		native = types.NewTx(&types.AccessListTx{
// 			Nonce:      wtx.Nonce(),
// 			GasPrice:   wtx.GasPrice(),
// 			Gas:        wtx.Gas(),
// 			To:         wtx.To(),
// 			Value:      wtx.Value(),
// 			Data:       wtx.Data(),
// 			V:          V,
// 			R:          R,
// 			S:          S,
// 			ChainID:    chainId,
// 			AccessList: wtx.AccessList(),
// 		})
// 	case types.DynamicFeeTxType:
// 		native = types.NewTx(&types.DynamicFeeTx{
// 			Nonce:      wtx.Nonce(),
// 			Gas:        wtx.Gas(),
// 			To:         wtx.To(),
// 			Value:      wtx.Value(),
// 			Data:       wtx.Data(),
// 			V:          V,
// 			R:          R,
// 			S:          S,
// 			ChainID:    chainId,
// 			AccessList: wtx.AccessList(),
// 			GasTipCap:  wtx.GasTipCap(),
// 			GasFeeCap:  wtx.GasFeeCap(),
// 		})
// 	case types.BlobTxType:
// 		native = types.NewTx(&types.BlobTx{
// 			Nonce:      wtx.Nonce(),
// 			Gas:        wtx.Gas(),
// 			Data:       wtx.Data(),
// 			AccessList: wtx.AccessList(),
// 			ChainID:    as256(tx.ChainID.Int()),
// 			V:          as256(V),
// 			R:          as256(R),
// 			S:          as256(S),
// 			GasTipCap:  as256(wtx.GasTipCap()),
// 			GasFeeCap:  as256(wtx.GasFeeCap()),
// 			To:         *wtx.To(),
// 			Value:      as256(wtx.Value()),
// 			BlobFeeCap: as256(wtx.BlobGasFeeCap()),
// 			BlobHashes: wtx.BlobHashes(),
// 			// ??
// 			Sidecar: nil,
// 		})
// 	default:
// 		return "", false, fmt.Errorf("unable to recover sender for transaction type %02x", txType)
// 	}

// 	signer := types.LatestSignerForChainID(tx.ChainID.Int())
// 	addr, err := signer.Sender(native)
// 	if err != nil {
// 		return "", false, err
// 	}
// 	return xc.Address(addr.String()), true, nil
// }
