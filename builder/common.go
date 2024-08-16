package builder

import xc "github.com/cordialsys/crosschain"

type TxCommonOptions struct {
	memo           *string
	timestamp      *int64
	gasFeePriority *xc.GasFeePriority
	publicKey      *[]byte
}
