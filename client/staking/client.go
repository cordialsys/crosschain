package staking

import (
	"context"

	xc "github.com/cordialsys/crosschain"
)

type State string

var Activating State = "activating"
var Activated State = "activated"
var Deactivating State = "deactivating"
var Inactive State = "inactive"

type Balance struct {
	State  State               `json:"state"`
	Amount xc.AmountBlockchain `json:"amount"`
}

type StakingClient interface {
	// What do we want to know about the stake account?
	// - state
	// - balance
	// - validator? credentials?
	// - illiquid balances?
	// A staking account can be identified using the following two names:
	// - chains/ETH/addresses/1234
	// - validators/x/accounts/y
	FetchStakeBalance(ctx context.Context, address xc.Address, validator string, stakeAccount xc.Address) ([]*Balance, error)

	// defintiely need:
	// - address (wallet)
	// - validator
	// - stakeAccount - do we need this???
	// - amount
	FetchStakeInput(ctx context.Context, address xc.Address, validator string, amount xc.AmountBlockchain) (xc.StakingInput, error)
}
