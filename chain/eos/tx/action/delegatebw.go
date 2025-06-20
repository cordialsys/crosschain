package action

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
)

func NewDelegateBW(fromAccount, toAccount string, cpuQuantity xc.AmountBlockchain, netQuantity xc.AmountBlockchain, transfer bool) (*eos.Action, error) {
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

	tf := DelegateBW{
		From:     eos.AccountName(fromAccount),
		Receiver: eos.AccountName(toAccount),
		// Amount:      totalAmountAsset,
		CPUQuantity: cpuAsset,
		NetQuantity: netAsset,
		Transfer:    transfer,
	}

	return &eos.Action{
		Account: eos.AccountName("eosio"),
		Name:    eos.ActN("delegatebw"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AccountName(fromAccount), Permission: eos.PermissionName("active")},
		},
		ActionData: eos.NewActionData(tf),
	}, nil
}

// The order of the fields is important:
type DelegateBW struct {
	From     eos.AccountName `json:"from"`
	Receiver eos.AccountName `json:"receiver"`
	// Amount      eos.Asset       `json:"amount"`
	NetQuantity eos.Asset `json:"stake_net_quantity"`
	CPUQuantity eos.Asset `json:"stake_cpu_quantity"`
	Transfer    bool      `json:"transfer"`

	// These seem to be output-only fields.
	// Amount xc.AmountHumanReadable `json:"amount,omitempty"`
	// Symbol string                 `json:"symbol,omitempty"`
}
