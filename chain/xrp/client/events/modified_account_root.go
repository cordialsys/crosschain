package events

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/client/types"
)

type EventModifiedAccountRoot struct {
	node *types.ModifiedNode
}

var _ Event = &EventModifiedAccountRoot{}

func (mnw *EventModifiedAccountRoot) GetAddress(txResponse *types.TransactionResponse) (xc.Address, error) {

	return xc.Address(mnw.node.FinalFields.Account), nil
}

func (mnw *EventModifiedAccountRoot) GetContract() (xc.ContractAddress, error) {
	return mnw.node.FinalFields.GetContract()
}

func (mnw *EventModifiedAccountRoot) GetAmount() (xc.AmountBlockchain, error) {
	transactedAmount, conversionErr := extractModifiedNodeBalance(mnw.node)
	if conversionErr != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to fetch ModifiedNode balance: %s", conversionErr.Error())
	}

	return transactedAmount, nil
}

func (mnw *EventModifiedAccountRoot) IsSource(txResponse *types.TransactionResponse) (bool, error) {
	if mnw.node.FinalFields == nil {
		return false, fmt.Errorf("empty FinalField in ModifiedNode")
	}

	if mnw.node.FinalFields.Account != txResponse.Result.Account {
		return false, nil
	} else {
		return true, nil
	}
}
