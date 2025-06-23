package action

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
)

func NewTransfer(fromAccount, toAccount string, quantity xc.AmountBlockchain, decimals int32, contractAccount string, contractSymbol string, memo string) (*eos.Action, error) {
	quantityString := NewAssetString(quantity, decimals, contractSymbol)
	asset, err := eos.NewAssetFromString(quantityString)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset: %v", err)
	}
	tf := Transfer{
		From:     eos.AccountName(fromAccount),
		To:       eos.AccountName(toAccount),
		Quantity: asset,
		Memo:     memo,
	}

	return &eos.Action{
		Account: eos.AccountName(contractAccount),
		Name:    eos.ActN("transfer"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AccountName(fromAccount), Permission: eos.PermissionName("active")},
		},
		ActionData: eos.NewActionData(tf),
	}, nil
}

type AssetString = string

// EOS requires the amount to be represented as a human string with 0 padded decimals.
func NewAssetString(amount xc.AmountBlockchain, decimals int32, symbol string) AssetString {
	human := amount.ToHuman(decimals)
	fixedString := human.Decimal().StringFixed(decimals)
	return AssetString(fmt.Sprintf("%s %s", fixedString, symbol))
}

// Transfer represents the `transfer` struct on `eosio.token` contract.
type Transfer struct {
	From     eos.AccountName `json:"from"`
	To       eos.AccountName `json:"to"`
	Quantity eos.Asset       `json:"quantity,omitempty"`
	Memo     string          `json:"memo"`

	// These seem to be output-only fields.
	Amount xc.AmountHumanReadable `json:"amount,omitempty"`
	Symbol string                 `json:"symbol,omitempty"`
}
