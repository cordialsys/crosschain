package events

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coming-chat/go-aptos/aptostypes"
)

type ParsedChange struct {
	Change              *aptostypes.Change
	Inner               *AptosChangeInner
	coinStoreChange     *CoinStoreChange
	objectCoreChange    *ObjectCoreChange
	fungibleStoreChange *FungibleStoreChange
}
type AptosChangeInner struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type ObjectCoreChange struct {
	AllowUnguatedTransfer bool            `json:"allow_ungated_transfer"`
	GuidCreationNumber    string          `json:"guid_creation_num"`
	Owner                 string          `json:"owner"`
	TransferEvents        json.RawMessage `json:"transfer_events"`
}

type FungibleStoreChange struct {
	Balance  string `json:"balance"`
	Frozen   bool   `json:"frozen"`
	Metadata struct {
		Inner string `json:"inner"`
	} `json:"metadata"`
}

type CoinStoreChange struct {
	DepositEvents  aptosEvents `json:"deposit_events"`
	WithdrawEvents aptosEvents `json:"withdraw_events"`
}
type aptosEvents struct {
	Counter string `json:"counter"`
	Guid    GuidId `json:"guid"`
}
type EventId struct {
	CreationNumber string `json:"creation_num"`
	AccountAddress string `json:"addr"`
}
type GuidId struct {
	Id EventId `json:"id"`
}

func ParseChange(ch *aptostypes.Change) (*ParsedChange, error) {
	parsed := &ParsedChange{
		Change: ch,
		Inner:  &AptosChangeInner{},
	}
	err := reserializeJson(ch.Data, parsed.Inner)
	if err != nil {
		return parsed, err
	}
	if strings.HasPrefix(parsed.Inner.Type, "0x1::coin::CoinStore") {
		change := &CoinStoreChange{}
		err := json.Unmarshal(parsed.Inner.Data, change)
		if err != nil {
			return parsed, fmt.Errorf("could not deserialize aptos change")
		}
		parsed.coinStoreChange = change
		return parsed, nil
	}
	if parsed.Inner.Type == "0x1::object::ObjectCore" {
		change := &ObjectCoreChange{}
		err := json.Unmarshal(parsed.Inner.Data, change)
		if err != nil {
			return parsed, fmt.Errorf("could not deserialize aptos change")
		}
		parsed.objectCoreChange = change
	}
	if parsed.Inner.Type == "0x1::fungible_asset::FungibleStore" {
		change := &FungibleStoreChange{}
		err := json.Unmarshal(parsed.Inner.Data, change)
		if err != nil {
			return parsed, fmt.Errorf("could not deserialize aptos change")
		}
		parsed.fungibleStoreChange = change
	}
	// unknown change type
	return parsed, nil
}
func (p *ParsedChange) AsCoinStore() (*CoinStoreChange, bool) {
	if p.coinStoreChange == nil {
		return nil, false
	}
	return p.coinStoreChange, true
}
func (p *ParsedChange) AsObjectCore() (*ObjectCoreChange, bool) {
	if p.objectCoreChange == nil {
		return nil, false
	}
	return p.objectCoreChange, true
}
func (p *ParsedChange) AsFungibleStore() (*FungibleStoreChange, bool) {
	if p.fungibleStoreChange == nil {
		return nil, false
	}
	return p.fungibleStoreChange, true
}
