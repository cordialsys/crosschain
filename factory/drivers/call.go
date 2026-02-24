package drivers

import (
	"encoding/json"
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/call"
	evmcall "github.com/cordialsys/crosschain/chain/evm/call"
	solanacall "github.com/cordialsys/crosschain/chain/solana/call"
)

var ErrCallNotSupported = errors.New("call not supported for this chain")

func NewCall(cfg *xc.ChainBaseConfig, method call.Method, msg json.RawMessage, signingAddress xc.Address) (xc.TxCall, error) {
	switch xc.Driver(cfg.Driver) {
	case xc.DriverEVM:
		return evmcall.NewCall(cfg, method, msg, signingAddress)
	case xc.DriverSolana:
		return solanacall.NewCall(cfg, method, msg, signingAddress)
	}
	return nil, fmt.Errorf("%w: %s", ErrCallNotSupported, string(cfg.Chain))
}
