package tx_input

import (
	xc "github.com/cordialsys/crosschain"
)

type KilnStakingInput struct {
	xc.StakingInputEnvelope

	PublicKeys  [][]byte            `json:"public_keys"`
	Credentials [][]byte            `json:"credentials"`
	Signatures  [][]byte            `json:"signatures"`
	Amount      xc.AmountBlockchain `json:"amount"`
}

func NewKilnStakingInput() *KilnStakingInput {
	return &KilnStakingInput{
		StakingInputEnvelope: *xc.NewStakingInputEnvelope(xc.StakingVariantEvmKiln),
	}
}
