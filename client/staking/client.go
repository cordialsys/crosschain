package staking

import (
	"context"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
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

	// Fetch staked balances accross different possible states
	FetchStakeBalance(ctx context.Context, address xc.Address, validator string, stakeAccount xc.Address) ([]*Balance, error)

	// Fetch inputs required for a staking transaction
	FetchStakingInput(ctx context.Context, args builder.StakeArgs) (xc.StakeTxInput, error)

	// Fetch inputs required for a unstaking transaction
	FetchUnstakingInput(ctx context.Context, args builder.StakeArgs) (xc.UnstakeTxInput, error)
}
