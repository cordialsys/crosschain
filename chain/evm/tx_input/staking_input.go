package tx_input

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/shopspring/decimal"
)

type KilnStakingInput struct {
	xc.StakingInputEnvelope
	TxInput
	// ContractAddress xc.ContractAddress `json:"contract_address"`
	PublicKeys [][]byte `json:"public_keys"`
	// Credentials     [][]byte           `json:"credentials"`
	Signatures [][]byte `json:"signatures"`
	// TODO this should be a 'argument-input'
	// Amount xc.AmountBlockchain `json:"amount"`
}

var _ xc.StakingInput = &KilnStakingInput{}

func NewKilnStakingInput() *KilnStakingInput {
	return &KilnStakingInput{
		StakingInputEnvelope: *xc.NewStakingInputEnvelope(xc.StakingVariantEvmKiln),
	}
}

func (stakingInput *KilnStakingInput) GetVariant() xc.StakingVariant {
	return stakingInput.Variant
}

func DivideAmount(chain *xc.ChainConfig, amount xc.AmountBlockchain) (uint64, error) {
	ethInc, _ := xc.NewAmountHumanReadableFromStr("32")
	weiInc := ethInc.ToBlockchain(chain.Decimals)

	if amount.Cmp(&weiInc) < 0 {
		return 0, fmt.Errorf("must stake at least 32 ether")
	}
	amountHuman := amount.ToHuman(chain.Decimals)

	quot := amountHuman.Div(ethInc)
	rounded := (decimal.Decimal)(quot).Round(0)
	if quot.String() != rounded.String() {
		return 0, fmt.Errorf("must stake an increment of 32 ether")
	}
	return quot.ToBlockchain(0).Uint64(), nil
}
