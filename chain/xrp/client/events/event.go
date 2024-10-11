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

			return &EventModifiedAccountRoot{node.ModifiedNode}, true, nil
		case "RippleState":
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
	var (
		newBalance xc.AmountHumanReadable
		decimals   int32
		err        error
	)

	if node.NewFields == nil {
		return xc.AmountBlockchain{}, fmt.Errorf("NewFields is empty")
	}

	if node.NewFields.Balance.XRPAmount != "" {
		decimals = types.XRP_NATIVE_DECIMALS
		newBalance, err = xc.NewAmountHumanReadableFromStr(node.NewFields.Balance.XRPAmount)
		if err != nil {
			return xc.AmountBlockchain{}, err
		}
	} else {
		decimals = types.TRUSTLINE_DECIMALS
		newBalance, err = xc.NewAmountHumanReadableFromStr(node.NewFields.Balance.TokenAmount.Value)
		if err != nil {
			return xc.AmountBlockchain{}, err
		}
	}

	return newBalance.ToBlockchain(decimals), nil
}

func extractModifiedNodeBalance(node *types.ModifiedNode) (xc.AmountBlockchain, error) {
	var (
		finalBalanceHumanReadable, previousBalanceHumanReadable xc.AmountHumanReadable
		finalFields, previousBalance                            xc.AmountBlockchain
		decimals                                                int32
		parseErr                                                error
	)

	if node.FinalFields == nil || node.PreviousFields == nil {
		return xc.AmountBlockchain{}, fmt.Errorf("FinalFields is empty")
	}

	if node.FinalFields.Balance.XRPAmount != "" {
		decimals = types.XRP_NATIVE_DECIMALS
		finalBalanceHumanReadable, parseErr = xc.NewAmountHumanReadableFromStr(node.FinalFields.Balance.XRPAmount)

		finalFields = finalBalanceHumanReadable.ToBlockchain(decimals)
	} else {
		decimals = types.TRUSTLINE_DECIMALS
		finalBalanceHumanReadable, parseErr = xc.NewAmountHumanReadableFromStr(node.FinalFields.Balance.TokenAmount.Value)
		finalFields = finalBalanceHumanReadable.ToBlockchain(decimals)
	}
	if parseErr != nil {
		return xc.AmountBlockchain{}, parseErr
	}

	if node.PreviousFields.Balance.XRPAmount != "" {
		previousBalanceHumanReadable, parseErr = xc.NewAmountHumanReadableFromStr(node.PreviousFields.Balance.XRPAmount)
		previousBalance = previousBalanceHumanReadable.ToBlockchain(decimals)
	} else {
		previousBalanceHumanReadable, parseErr = xc.NewAmountHumanReadableFromStr(node.PreviousFields.Balance.TokenAmount.Value)
		previousBalance = previousBalanceHumanReadable.ToBlockchain(decimals)
	}
	if parseErr != nil {
		return xc.AmountBlockchain{}, parseErr
	}

	transactedAmount := previousBalance.Sub(&finalFields)

	return transactedAmount.Abs(), nil
}
