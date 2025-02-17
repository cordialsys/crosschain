package events

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/client/types"
	"github.com/shopspring/decimal"
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
			return "", fmt.Errorf("empty LowLimit in FinalFields")
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

	var (
		accountAddress      string
		modifiedNodeAddress string
		balanceWentDown     bool
	)

	finalBalanceHuman, err := xc.NewAmountHumanReadableFromStr(mnw.node.FinalFields.Balance.TokenAmount.Value)
	if err != nil {
		return false, err
	}

	previousBalanceHuamn, err := xc.NewAmountHumanReadableFromStr(mnw.node.PreviousFields.Balance.TokenAmount.Value)
	if err != nil {
		return false, err
	}

	finalBalanceDecimal := decimal.Decimal.Abs(finalBalanceHuman.Decimal())
	previousBalanceDecimal := decimal.Decimal.Abs(previousBalanceHuamn.Decimal())

	balance := previousBalanceDecimal.Sub(finalBalanceDecimal)

	balanceWentDown = previousBalanceDecimal.Cmp(finalBalanceDecimal) > 0

	accountAddress = txResponse.Result.Account
	if balance.IsPositive() {
		modifiedNodeAddress = mnw.node.FinalFields.HighLimit.Issuer
	}

	if balance.IsNegative() {
		modifiedNodeAddress = mnw.node.FinalFields.LowLimit.Issuer
	}

	// Compare the modified address with the account (sender) address from the transaction
	isSourceAccount := accountAddress == modifiedNodeAddress

	if balanceWentDown && isSourceAccount {
		return true, nil // It's the source
	} else if !balanceWentDown && !isSourceAccount {
		return false, nil // It's the destination
	}

	return balanceWentDown, nil
}
