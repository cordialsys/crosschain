package tempo

import (
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	evmbuilder "github.com/cordialsys/crosschain/chain/evm/builder"
)

type TxBuilder struct {
	evmbuilder.TxBuilder
}

func NewTxBuilder(cfg *xc.ChainBaseConfig) (TxBuilder, error) {
	evmBuilder, err := evmbuilder.NewTxBuilder(cfg)
	if err != nil {
		return TxBuilder{}, err
	}

	return TxBuilder{
		TxBuilder: evmBuilder,
	}, nil
}

var _ xcbuilder.FullBuilder = &TxBuilder{}
