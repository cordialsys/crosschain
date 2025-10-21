package errors

import (
	e "errors"
)

var ErrStakingAmountRequired = e.New("staking amount is required")
var ErrStakingAmountNotUsed = e.New("staking amount should be removed as it is not used")
