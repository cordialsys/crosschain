package action

import (
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
)

func NewSellRam(account string, bytes uint64) (*eos.Action, error) {
	sr := SellRam{
		Account: eos.AccountName(account),
		Bytes:   bytes,
	}

	return &eos.Action{
		Account: eos.AccountName("eosio"),
		Name:    eos.ActN("sellram"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AccountName(account), Permission: eos.PermissionName("active")},
		},
		ActionData: eos.NewActionData(sr),
	}, nil
}

// SellRam represents the `sellram` struct on the `eosio` contract.
type SellRam struct {
	Account eos.AccountName `json:"account"`
	Bytes   uint64          `json:"bytes"`
}
