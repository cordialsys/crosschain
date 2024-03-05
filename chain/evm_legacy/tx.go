package evm_legacy

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm"
)

// Tx for EVM
type Tx = evm.Tx

var _ xc.Tx = &Tx{}
