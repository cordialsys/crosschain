package events

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/client/types"
)

type eventDeletedAccountRoot struct {
	node *types.DeletedNode
}

var _ Event = &eventDeletedAccountRoot{}

func (mnw *eventDeletedAccountRoot) GetAddress(txResponse *types.TransactionResponse) (xc.Address, error) {
	return xc.Address(mnw.node.FinalFields.Account), nil
}

func (mnw *eventDeletedAccountRoot) GetContract() (xc.ContractAddress, error) {
	return mnw.node.FinalFields.GetContract()
}

func (mnw *eventDeletedAccountRoot) GetAmount() (xc.AmountBlockchain, error) {
	transactedAmount, conversionErr := extractModifiedNodeBalance(mnw.node.FinalFields, mnw.node.PreviousFields)
	if conversionErr != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to fetch ModifiedNode balance: %s", conversionErr.Error())
	}

	return transactedAmount, nil
}

func (mnw *eventDeletedAccountRoot) IsSource(txResponse *types.TransactionResponse) (bool, error) {
	// A deleted account is always a source, it doesn't receive funds
	return true, nil
}
