package action

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
)

func NewUnDelegateBW(fromAccount, toAccount string, cpuQuantity xc.AmountBlockchain, netQuantity xc.AmountBlockchain) (*eos.Action, error) {

	cpuQ := NewAssetString(cpuQuantity, 4, "EOS")
	netQ := NewAssetString(netQuantity, 4, "EOS")

	cpuAsset, err := eos.NewAssetFromString(cpuQ)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset: %v", err)
	}
	netAsset, err := eos.NewAssetFromString(netQ)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset: %v", err)
	}
	tf := UnDelegateBW{
		From:        eos.AccountName(fromAccount),
		Receiver:    eos.AccountName(toAccount),
		CPUQuantity: cpuAsset,
		NetQuantity: netAsset,
	}

	return &eos.Action{
		Account: eos.AccountName("eosio"),
		Name:    eos.ActN("undelegatebw"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AccountName(fromAccount), Permission: eos.PermissionName("active")},
		},
		ActionData: eos.NewActionData(tf),
	}, nil
}

type UnDelegateBW struct {
	From        eos.AccountName `json:"from"`
	Receiver    eos.AccountName `json:"receiver"`
	NetQuantity eos.Asset       `json:"unstake_net_quantity,omitempty"`
	CPUQuantity eos.Asset       `json:"unstake_cpu_quantity,omitempty"`
}
