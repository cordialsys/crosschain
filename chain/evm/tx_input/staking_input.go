package tx_input

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/address"
	"github.com/shopspring/decimal"
)

type KilnStakingInput struct {
	xc.StakingInputEnvelope

	ContractAddress xc.ContractAddress `json:"contract_address"`
	PublicKeys      [][]byte           `json:"public_keys"`
	Credentials     [][]byte           `json:"credentials"`
	Signatures      [][]byte           `json:"signatures"`
	// TODO this should be a 'argument-input'
	Amount xc.AmountBlockchain `json:"amount"`
}

var _ xc.StakingInput = &KilnStakingInput{}

func NewKilnStakingInput() *KilnStakingInput {
	return &KilnStakingInput{
		StakingInputEnvelope: *xc.NewStakingInputEnvelope(xc.StakingVariantEvmKiln),
	}
}

func (stakingInput *KilnStakingInput) SetOwner(addr xc.Address) error {
	addrGeth, err := address.FromHex(addr)
	if err != nil {
		return err
	}

	addrBz := addrGeth.Bytes()
	withdrawCred := [32]byte{}
	copy(withdrawCred[32-len(addrBz):], addrBz)
	// set the credential type
	withdrawCred[0] = 1

	// Set the withdrawal credentials
	if len(stakingInput.PublicKeys) == 0 {
		return fmt.Errorf("no validator public keys set in input yet")
	}
	stakingInput.Credentials = make([][]byte, len(stakingInput.PublicKeys))

	for i := range stakingInput.Credentials {
		stakingInput.Credentials[i] = withdrawCred[:]
	}

	return nil
}
func (stakingInput *KilnStakingInput) SetContract(contract xc.ContractAddress) error {
	_, err := address.FromHex(xc.Address(contract))
	if err != nil {
		return err
	}
	stakingInput.ContractAddress = contract
	return nil
}

// func (stakingInput *KilnStakingInput) SetAmount(amount xc.AmountBlockchain) error {
// 	if err := ValidateAmount(amount); err != nil {
// 		return err
// 	}
// 	stakingInput.Amount = amount
// }

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
