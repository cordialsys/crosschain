package events

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/client/types"
)

type Event interface {
	// IsValidMovement() bool
	GetAddress(txResponse *types.TransactionResponse) (xc.Address, error)
	GetContract() (xc.ContractAddress, error)
	GetAmount() (xc.AmountBlockchain, error)
	IsSource(txResponse *types.TransactionResponse) (bool, error)
}

func NewEvent(node types.AffectedNodes) (Event, bool, error) {
	if node.ModifiedNode != nil {
		if node.ModifiedNode.FinalFields == nil {
			return nil, false, fmt.Errorf("empty FinalFields in ModifiedNode")
		}
		switch node.ModifiedNode.LedgerEntryType {
		case "AccountRoot":
			if !node.ModifiedNode.PreviousFields.Balance.Valid() {
				// skip
				return nil, false, nil
			}
			return &EventModifiedAccountRoot{node.ModifiedNode}, true, nil
		case "RippleState":
			if !node.ModifiedNode.PreviousFields.Balance.Valid() {
				// skip
				return nil, false, nil
			}
			return &eventModifiedRippleState{node.ModifiedNode}, true, nil
		default:
			// skip
			return nil, false, nil
		}
	} else if node.CreatedNode != nil {

		if node.CreatedNode.NewFields == nil {
			return nil, false, fmt.Errorf("empty NewFields in CreatedNode")
		}
		switch node.CreatedNode.LedgerEntryType {
		case "AccountRoot":

			return &eventCreatedAccountRoot{node.CreatedNode}, true, nil
		case "RippleState":
			return &eventCreatedRippleState{node.CreatedNode}, true, nil
		default:
			// skip
			return nil, false, nil
		}
	} else if node.DeletedNode != nil {
		switch node.DeletedNode.LedgerEntryType {
		case "AccountRoot":
			return &eventDeletedAccountRoot{node.DeletedNode}, true, nil
		default:
			// skip
			return nil, false, nil
		}
	}
	return nil, false, nil
}

func extractCreatedNodeBalance(node *types.CreatedNode) (xc.AmountBlockchain, error) {
	if node.NewFields == nil {
		return xc.AmountBlockchain{}, fmt.Errorf("NewFields is empty")
	}

	if node.NewFields.Balance.XRPAmount != "" {
		return xc.NewAmountBlockchainFromStr(node.NewFields.Balance.XRPAmount), nil
	} else {
		// XRP node reports token balance adjusted for decimals
		tokenValue, err := xc.NewAmountHumanReadableFromStr(node.NewFields.Balance.TokenAmount.Value)
		if err != nil {
			return xc.AmountBlockchain{}, err
		}
		return tokenValue.ToBlockchain(types.TRUSTLINE_DECIMALS), nil
	}

}

func extractModifiedNodeBalance(finalFields *types.FinalFields, previousFields *types.PreviousFields) (xc.AmountBlockchain, error) {
	var (
		finalBalance, previousBalance xc.AmountBlockchain
	)

	if finalFields == nil || previousFields == nil {
		return xc.AmountBlockchain{}, fmt.Errorf("FinalFields is empty")
	}

	if finalFields.Balance.XRPAmount != "" {
		finalBalance = xc.NewAmountBlockchainFromStr(finalFields.Balance.XRPAmount)
	} else {
		// XRP node reports token balance adjusted for decimals
		tokenValue, err := xc.NewAmountHumanReadableFromStr(finalFields.Balance.TokenAmount.Value)
		if err != nil {
			return xc.AmountBlockchain{}, err
		}
		finalBalance = tokenValue.ToBlockchain(types.TRUSTLINE_DECIMALS)
	}

	if previousFields.Balance.XRPAmount != "" {
		previousBalance = xc.NewAmountBlockchainFromStr(previousFields.Balance.XRPAmount)
	} else {
		// XRP node reports token balance adjusted for decimals
		tokenValue, err := xc.NewAmountHumanReadableFromStr(previousFields.Balance.TokenAmount.Value)
		if err != nil {
			return xc.AmountBlockchain{}, err
		}
		previousBalance = tokenValue.ToBlockchain(types.TRUSTLINE_DECIMALS)
	}

	transactedAmount := previousBalance.Sub(&finalBalance)

	return transactedAmount.Abs(), nil
}
