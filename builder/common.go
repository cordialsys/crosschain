package builder

import xc "github.com/cordialsys/crosschain"

type TxCommonOptions struct {
	memo           *string
	timestamp      *int64
	gasFeePriority *xc.GasFeePriority
	publicKey      *[]byte
}

func get[T any](arg *T) (T, bool) {
	if arg == nil {
		var zero T
		return zero, false
	}
	return *arg, true
}

func (opts *TxCommonOptions) GetMemo() (string, bool)                { return get(opts.memo) }
func (opts *TxCommonOptions) GetTimestamp() (int64, bool)            { return get(opts.timestamp) }
func (opts *TxCommonOptions) GetPriority() (xc.GasFeePriority, bool) { return get(opts.gasFeePriority) }
func (opts *TxCommonOptions) GetPublicKey() ([]byte, bool)           { return get(opts.publicKey) }
