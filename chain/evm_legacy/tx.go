package evm_legacy

import (
	xc "github.com/cordialsys/crosschain"
	evmtx "github.com/cordialsys/crosschain/chain/evm/tx"
)

// Tx for EVM
type Tx = evmtx.Tx

var _ xc.Tx = &Tx{}
