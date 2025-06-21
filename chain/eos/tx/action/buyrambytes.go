package action

import (
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
)

func NewBuyRamBytes(payer, receiver string, bytes uint32) (*eos.Action, error) {
	brb := BuyRamBytes{
		Payer:    eos.AccountName(payer),
		Receiver: eos.AccountName(receiver),
		Bytes:    bytes,
	}

	return &eos.Action{
		Account: eos.AccountName("eosio"),
		Name:    eos.ActN("buyrambytes"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AccountName(payer), Permission: eos.PermissionName("active")},
		},
		ActionData: eos.NewActionData(brb),
	}, nil
}

// BuyRamBytes represents the `buyrambytes` struct on the `eosio` contract.
type BuyRamBytes struct {
	Payer    eos.AccountName `json:"payer"`
	Receiver eos.AccountName `json:"receiver"`
	Bytes    uint32          `json:"bytes"`
}
