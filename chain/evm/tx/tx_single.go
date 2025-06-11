package tx

import (
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum/core/types"
)

type SingleTx struct {
	args  xcbuilder.TransferArgs
	input *tx_input.TxInput
	chain *xc.ChainBaseConfig

	signature xc.TxSignature
}

var _ evmTx = &SingleTx{}

func NewSingleTx(args xcbuilder.TransferArgs, input *tx_input.TxInput, chain *xc.ChainBaseConfig) *SingleTx {
	return &SingleTx{
		args,
		input,
		chain,
		xc.TxSignature{},
	}
}

func (tx *SingleTx) BuildEthTx() (*types.Transaction, error) {
	destination, data, err := EvmDestinationAndData(tx.args)
	if err != nil {
		return nil, err
	}
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:   tx.input.ChainId.Int(),
		Nonce:     tx.input.Nonce,
		GasTipCap: tx.input.GasTipCap.Int(),
		GasFeeCap: tx.input.GasFeeCap.Int(),
		Gas:       tx.input.GasLimit,
		To:        &destination,
		Value:     tx.args.GetAmount().Int(),
		Data:      data,
	}), nil
}

func (tx *SingleTx) Sighashes() ([]*xc.SignatureRequest, error) {
	ethTx, err := tx.BuildEthTx()
	if err != nil {
		return nil, err
	}
	sighash := GetEthSigner(tx.chain, tx.input).Hash(ethTx).Bytes()
	return []*xc.SignatureRequest{xc.NewSignatureRequest(sighash)}, nil
}

func (tx *SingleTx) AddSignatures(signatures []*xc.SignatureResponse) {
	tx.signature = signatures[0].Signature
}

func (tx *SingleTx) Serialize() ([]byte, error) {
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
