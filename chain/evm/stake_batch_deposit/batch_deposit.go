package stake_batch_deposit

import (
	_ "embed"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

//go:embed abi.json
var abiJson string

func NewAbi() abi.ABI {
	batchDeposit, err := abi.JSON(strings.NewReader(abiJson))
	if err != nil {
		panic(err)
	}
	return batchDeposit
}

func init() {
	NewAbi()
}
