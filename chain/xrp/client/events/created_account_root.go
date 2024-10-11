package events

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/client/types"
)

type eventCreatedAccountRoot struct {
	node *types.CreatedNode
}

var _ Event = &eventCreatedAccountRoot{}

func (mnw *eventCreatedAccountRoot) GetAddress(txResponse *types.TransactionResponse) (xc.Address, error) {
	return xc.Address(mnw.node.NewFields.Account), nil
}

func (mnw *eventCreatedAccountRoot) GetContract() (xc.ContractAddress, error) {
	return mnw.node.NewFields.GetContract()
}

func (mnw *eventCreatedAccountRoot) GetAmount() (xc.AmountBlockchain, error) {
	transactedAmount, conversionErr := extractCreatedNodeBalance(mnw.node)
	if conversionErr != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to fetch CreatedNode balance: %s", conversionErr.Error())
	}

	return transactedAmount, nil
}

func (mnw *eventCreatedAccountRoot) IsSource(txResponse *types.TransactionResponse) (bool, error) {
	if mnw.node.NewFields == nil {
		return false, fmt.Errorf("empty NewFields in CreatedNode")
	}

	if mnw.node.NewFields.Account != txResponse.Result.Account {
		return false, nil
	} else {
		return true, nil
	}
}
