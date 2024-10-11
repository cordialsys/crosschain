package events

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/client/types"
)

type eventCreatedRippleState struct {
	node *types.CreatedNode
}

var _ Event = &eventCreatedRippleState{}

func (mnw *eventCreatedRippleState) GetAddress(txResponse *types.TransactionResponse) (xc.Address, error) {

	isSource, fetchIsSourceErr := mnw.IsSource(txResponse)
	if fetchIsSourceErr != nil {
		return "", fetchIsSourceErr
	}

	if isSource {
		if mnw.node.NewFields.LowLimit == nil {
			return "", fmt.Errorf("empty HighLimit in NewFields")
		}

		return xc.Address(mnw.node.NewFields.LowLimit.Issuer), nil
	} else {

		if mnw.node.NewFields.HighLimit == nil {
			return "", fmt.Errorf("empty HighLimit in NewFields")
		}

		return xc.Address(mnw.node.NewFields.HighLimit.Issuer), nil
	}

}

func (mnw *eventCreatedRippleState) GetContract() (xc.ContractAddress, error) {
	return mnw.node.NewFields.GetContract()
}

func (mnw *eventCreatedRippleState) GetAmount() (xc.AmountBlockchain, error) {

	transactedAmount, conversionErr := extractCreatedNodeBalance(mnw.node)
	if conversionErr != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to fetch CreatedNode balance: %s", conversionErr.Error())
	}

	return transactedAmount, nil
}

func (mnw *eventCreatedRippleState) IsSource(txResponse *types.TransactionResponse) (bool, error) {

	finalBalanceHumanReadable, err := xc.NewAmountHumanReadableFromStr(mnw.node.NewFields.Balance.TokenAmount.Value)
	if err != nil {
		return false, err
	}

	finalBalanceBlockchain := finalBalanceHumanReadable.ToBlockchain(6)
	zero := xc.NewAmountBlockchainFromUint64(0)

	if finalBalanceBlockchain.Cmp(&zero) < 0 {
		if mnw.node.NewFields.HighLimit == nil {
			return false, fmt.Errorf("empty HighLimit in NewFields")
		}

		return false, nil
	} else {
		if mnw.node.NewFields.LowLimit == nil {
			return false, fmt.Errorf("empty LowLimit in NewFields")
		}

		return true, nil
	}

}
