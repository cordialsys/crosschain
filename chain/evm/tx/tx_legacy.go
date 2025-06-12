package tx

import (
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum/core/types"
)

type LegacyTx struct {
	args  xcbuilder.TransferArgs
	input *tx_input.TxInput
	chain *xc.ChainBaseConfig

	signature xc.TxSignature
}

var _ evmTx = &LegacyTx{}

func NewLegacyTx(args xcbuilder.TransferArgs, input *tx_input.TxInput, chain *xc.ChainBaseConfig) *LegacyTx {
	return &LegacyTx{
		args,
		input,
		chain,
		xc.TxSignature{},
	}
}

func (tx *LegacyTx) BuildEthTx() (*types.Transaction, error) {
	destination, amount, data, err := EvmDestinationAndAmountAndData(tx.args)
	if err != nil {
		return nil, err
	}
	ethTx := types.NewTransaction(
		tx.input.Nonce,
		destination,
		amount,
		tx.input.GasLimit,
		tx.input.GasPrice.Int(),
		data,
	)
	if len(tx.signature) > 0 {
		ethTx, err = ethTx.WithSignature(GetEthSigner(tx.chain, tx.input), tx.signature)
		if err != nil {
			return nil, err
		}
	}
	return ethTx, nil
}

func (tx *LegacyTx) Sighashes() ([]*xc.SignatureRequest, error) {
	ethTx, err := tx.BuildEthTx()
	if err != nil {
		return nil, err
	}
	sighash := GetEthSigner(tx.chain, tx.input).Hash(ethTx).Bytes()
	return []*xc.SignatureRequest{xc.NewSignatureRequest(sighash)}, nil
}

func (tx *LegacyTx) AdditionalSighashes() ([]*xc.SignatureRequest, error) {
	return nil, nil
}

func (tx *LegacyTx) AddSignatures(signatures []*xc.SignatureResponse) {
	tx.signature = signatures[0].Signature
}

func (tx *LegacyTx) Serialize() ([]byte, error) {
	ethTx, err := tx.BuildEthTx()
	if err != nil {
		return nil, err
	}
	return ethTx.MarshalBinary()
}
