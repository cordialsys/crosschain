package action

import (
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
)

func NewNewAccount(creator, name string, owner, active eos.Authority) (*eos.Action, error) {
	na := NewAccount{
		Creator: eos.AccountName(creator),
		Name:    eos.AccountName(name),
		Owner:   owner,
		Active:  active,
	}

	return &eos.Action{
		Account: eos.AccountName("eosio"),
		Name:    eos.ActN("newaccount"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AccountName(creator), Permission: eos.PermissionName("active")},
		},
		ActionData: eos.NewActionData(na),
	}, nil
}

type NewAccount struct {
	Creator eos.AccountName `json:"creator"`
	Name    eos.AccountName `json:"name"`
	Owner   eos.Authority   `json:"owner"`
	Active  eos.Authority   `json:"active"`
}
