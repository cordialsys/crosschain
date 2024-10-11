package events

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/client/types"
)

type eventModifiedRippleState struct {
	node *types.ModifiedNode
}

var _ Event = &eventModifiedRippleState{}

func (mnw *eventModifiedRippleState) GetAddress(txResponse *types.TransactionResponse) (xc.Address, error) {

	isSource, fetchIsSourceErr := mnw.IsSource(txResponse)
	if fetchIsSourceErr != nil {
		return "", fetchIsSourceErr
	}

	if isSource {
		if mnw.node.FinalFields.LowLimit == nil {
			return "", fmt.Errorf("empty HighLimit in FinalFields")
		}

		return xc.Address(mnw.node.FinalFields.LowLimit.Issuer), nil
	} else {
		if mnw.node.FinalFields.HighLimit == nil {
			return "", fmt.Errorf("empty HighLimit in FinalFields")
		}

		return xc.Address(mnw.node.FinalFields.HighLimit.Issuer), nil
	}
}

func (mnw *eventModifiedRippleState) GetContract() (xc.ContractAddress, error) {
	return mnw.node.FinalFields.GetContract()
}

func (mnw *eventModifiedRippleState) GetAmount() (xc.AmountBlockchain, error) {
	transactedAmount, conversionErr := extractModifiedNodeBalance(mnw.node)
	if conversionErr != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to fetch ModifiedNode balance: %s", conversionErr.Error())
	}

	return transactedAmount, nil
}

func (mnw *eventModifiedRippleState) IsSource(txResponse *types.TransactionResponse) (bool, error) {

	finalBalanceHuman, err := xc.NewAmountHumanReadableFromStr(mnw.node.FinalFields.Balance.TokenAmount.Value)
	if err != nil {
		return false, err
	}

	previousBalanceHuamn, err := xc.NewAmountHumanReadableFromStr(mnw.node.PreviousFields.Balance.TokenAmount.Value)
	if err != nil {
		return false, err
	}

	// use max precision just for the comparison
	finalBalance := finalBalanceHuman.ToBlockchain(15)
	previousBalance := previousBalanceHuamn.ToBlockchain(15)

	// If the balance goes down, this must be a source address.
	balanceWentDown := previousBalance.Cmp(&finalBalance) > 0

	// TODO this is not always correct.. I think we must also consider if the balance is for the account address
	// or the destination address (not the account address).

	return !balanceWentDown, nil
}
