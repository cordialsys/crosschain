package tempo

import (
	"github.com/cordialsys/crosschain/chain/evm"
	"github.com/cordialsys/crosschain/client/errors"
)

// CheckError delegates to EVM error checking
func CheckError(err error) errors.Status {
	return evm.CheckError(err)
}
