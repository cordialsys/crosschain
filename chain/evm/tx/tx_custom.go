package tx

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum/core/types"
)

type CustomTx struct {
	// args  xcbuilder.TransferArgs
	// input *tx_input.TxInput
	chain *xc.ChainBaseConfig
	input *tx_input.TxInput
	ethTx *types.Transaction

	signature xc.TxSignature
}

var _ evmTx = &SingleTx{}

func newCustomTxInner(input *tx_input.TxInput, chain *xc.ChainBaseConfig, ethTx *types.Transaction) *CustomTx {
	return &CustomTx{
		chain,
		input,
		ethTx,
		xc.TxSignature{},
	}
}

func NewCustomTx(input *tx_input.TxInput, chain *xc.ChainBaseConfig, ethTx *types.Transaction) xc.Tx {
	return &Tx{
		txInner: newCustomTxInner(input, chain, ethTx),
	}
}

func (tx *CustomTx) BuildEthTx() (*types.Transaction, error) {
	return tx.ethTx, nil
}

func (tx *CustomTx) Sighashes() ([]*xc.SignatureRequest, error) {
	ethTx, err := tx.BuildEthTx()
	if err != nil {
		return nil, err
	}
	sighash := GetEthSigner(tx.chain, tx.input).Hash(ethTx).Bytes()
	return []*xc.SignatureRequest{xc.NewSignatureRequest(sighash)}, nil
}

func (tx *CustomTx) AdditionalSighashes() ([]*xc.SignatureRequest, error) {
	return nil, nil
}

func (tx *CustomTx) AddSignatures(signatures []*xc.SignatureResponse) {
	tx.signature = signatures[0].Signature
}

func (tx *CustomTx) Serialize() ([]byte, error) {
	ethTx, err := tx.BuildEthTx()
	if err != nil {
		return nil, err
	}
	signedTx, err := ethTx.WithSignature(GetEthSigner(tx.chain, tx.input), tx.signature)
	if err != nil {
		return nil, err
	}
	return signedTx.MarshalBinary()
}
