package subscan

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate/client/api"
	"github.com/cordialsys/crosschain/client/tx_info"
)

type SubscanExtrinsicResponse struct {
	Code        int                  `json:"code"`
	Message     string               `json:"message"`
	GeneratedAt int64                `json:"generated_at"`
	Data        SubscanExtrinsicData `json:"data"`
}

type SubscanExtrinsicData struct {
	BlockTimestamp     int64          `json:"block_timestamp"`
	BlockNum           int64          `json:"block_num"`
	ExtrinsicIndex     string         `json:"extrinsic_index"`
	CallModuleFunction string         `json:"call_module_function"`
	CallModule         string         `json:"call_module"`
	AccountID          string         `json:"account_id"`
	Signature          string         `json:"signature"`
	Nonce              int            `json:"nonce"`
	ExtrinsicHash      string         `json:"extrinsic_hash"`
	Success            bool           `json:"success"`
	Params             []Param        `json:"params"`
	Transfer           interface{}    `json:"transfer"` // Assuming it can be null
	Event              []*Event       `json:"event"`
	EventCount         int            `json:"event_count"`
	Fee                string         `json:"fee"`
	FeeUsed            string         `json:"fee_used"`
	Error              interface{}    `json:"error"` // Assuming it can be null
	Finalized          bool           `json:"finalized"`
	Lifetime           interface{}    `json:"lifetime"` // Assuming it can be null
	Tip                string         `json:"tip"`
	AccountDisplay     AccountDisplay `json:"account_display"`
	BlockHash          string         `json:"block_hash"`
	Pending            bool           `json:"pending"`
	SubCalls           interface{}    `json:"sub_calls"` // Assuming it can be null
}

type Param struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	TypeName string      `json:"type_name"`
	Value    interface{} `json:"value"`
}

type Event struct {
	EventIndex     string          `json:"event_index"`
	BlockNum       int64           `json:"block_num"`
	ExtrinsicIdx   int             `json:"extrinsic_idx"`
	ModuleID       string          `json:"module_id"`
	EventID        string          `json:"event_id"`
	Params         json.RawMessage `json:"params"`
	Phase          int             `json:"phase"`
	EventIdx       int             `json:"event_idx"`
	ExtrinsicHash  string          `json:"extrinsic_hash"`
	Finalized      bool            `json:"finalized"`
	BlockTimestamp int64           `json:"block_timestamp"`

	parsedParams []*Param `json:"-"`
}

var _ api.EventI = &Event{}

func (ev *Event) GetEventDescriptor() (*txinfo.Event, bool) {
	return txinfo.NewEvent(ev.EventIndex, txinfo.MovementVariantNative), true
}

// these pesky events can be reported as:
// - expected type, json encoded
// - encoded as an embedded json string
// - empty string
func (ev *Event) ParseParams() ([]*Param, error) {
	var asJsonString string
	var asParams = []*Param{}
	if len(ev.Params) == 0 {
		return nil, nil
	}
	err1 := json.Unmarshal(ev.Params, &asParams)
	if err1 != nil {
		// try as embedded JSON string
		err := json.Unmarshal(ev.Params, &asJsonString)
		if err != nil {
			return nil, err1
		}
		if string(asJsonString) == "" {
			return nil, nil
		}
		asParams = []*Param{}
		err2 := json.Unmarshal([]byte(asJsonString), &asParams)
		ev.parsedParams = asParams
		if err2 == nil {
			return ev.parsedParams, nil
		}
		return ev.parsedParams, errors.Join(err1, err2)
	} else {
		ev.parsedParams = asParams
	}
	return ev.parsedParams, nil
}

type AccountDisplay struct {
	Address string `json:"address"`
}

func (ev *Event) GetId() string {
	return ev.EventID
}
func (ev *Event) GetModule() string {
	return ev.ModuleID
}
func (ev *Event) GetParam(name string, index int) (interface{}, bool) {
	for _, p := range ev.parsedParams {
		if p.Name == name {
			return p.Value, true
		}
	}
	return nil, false
}

func GetParam[T any](ev *Event, name string) (T, error) {
	for _, p := range ev.parsedParams {
		if p.Name == name {
			value, ok := p.Value.(T)
			if !ok {
				return value, fmt.Errorf("unexpected type for event %s.%s param %s, recieved %T but expected %T", ev.ModuleID, ev.EventID, name, p.Value, value)
			}
			return value, nil
		}
	}
	var zero T
	return zero, fmt.Errorf("did not find expected parameter %s on event %s.%s", name, ev.ModuleID, ev.EventID)
}

func GetParamInt(ev *Event, name string) (xc.AmountBlockchain, error) {
	val, err := GetParam[string](ev, name)
	if err != nil {
		return xc.AmountBlockchain{}, err
	}
	return xc.NewAmountBlockchainFromStr(val), nil
}

func GetParamAccountId(ev *Event, name string) ([]byte, error) {
	val, err := GetParam[string](ev, name)
	if err != nil {
		return nil, err
	}
	return codec.HexDecodeString(val)
}
