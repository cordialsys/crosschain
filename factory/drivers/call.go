package drivers

import (
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	evmcall "github.com/cordialsys/crosschain/chain/evm/call"
	solanacall "github.com/cordialsys/crosschain/chain/solana/call"
)

func NewCall(cfg *xc.ChainBaseConfig, msg json.RawMessage) (xc.TxCall, error) {
	switch xc.Driver(cfg.Driver) {
	case xc.DriverEVM:
		return evmcall.NewCall(cfg, msg)
	case xc.DriverSolana:
		return solanacall.NewCall(cfg, msg)
	}
	return nil, fmt.Errorf("no call resource defined for: %s", string(cfg.Chain))
}
