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
	}
	// skip
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

func extractModifiedNodeBalance(node *types.ModifiedNode) (xc.AmountBlockchain, error) {
	var (
		finalFields, previousBalance xc.AmountBlockchain
	)

	if node.FinalFields == nil || node.PreviousFields == nil {
		return xc.AmountBlockchain{}, fmt.Errorf("FinalFields is empty")
	}

	if node.FinalFields.Balance.XRPAmount != "" {
		finalFields = xc.NewAmountBlockchainFromStr(node.FinalFields.Balance.XRPAmount)
	} else {
		// XRP node reports token balance adjusted for decimals
		tokenValue, err := xc.NewAmountHumanReadableFromStr(node.FinalFields.Balance.TokenAmount.Value)
		if err != nil {
			return xc.AmountBlockchain{}, err
		}
		finalFields = tokenValue.ToBlockchain(types.TRUSTLINE_DECIMALS)
	}

	if node.PreviousFields.Balance.XRPAmount != "" {
		previousBalance = xc.NewAmountBlockchainFromStr(node.PreviousFields.Balance.XRPAmount)
	} else {
		// XRP node reports token balance adjusted for decimals
		tokenValue, err := xc.NewAmountHumanReadableFromStr(node.PreviousFields.Balance.TokenAmount.Value)
		if err != nil {
			return xc.AmountBlockchain{}, err
		}
		previousBalance = tokenValue.ToBlockchain(types.TRUSTLINE_DECIMALS)
	}

	transactedAmount := previousBalance.Sub(&finalFields)

	return transactedAmount.Abs(), nil
}
