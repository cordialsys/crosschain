package rpc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/centrifuge/go-substrate-rpc-client/v4/registry"
	"github.com/centrifuge/go-substrate-rpc-client/v4/registry/parser"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/cordialsys/crosschain/chain/substrate/client/api"
	"github.com/sirupsen/logrus"
)

type Event struct {
	Module string
	Event  string
	Raw    *parser.Event
}

var _ api.EventI = &Event{}

func NewEvent(raw *parser.Event) *Event {
	parts := strings.Split(raw.Name, ".")
	module := parts[0]
	event := ""
	if len(parts) > 1 {
		event = parts[1]
	}
	return &Event{module, event, raw}
}

// Or may be called "pallet"
func (ev *Event) GetModule() string {
	return ev.Module
}

// Or may just be the "name"
func (ev *Event) GetEvent() string {
	return ev.Event
}

func unwrap(value *registry.DecodedField) (decoded interface{}) {
	// convert to native types as needed
	switch value := value.Value.(type) {
	case registry.DecodedFields:
		// extra dimension turns up sometimes that we need to flatten
		if len(value) > 0 {
			return value[0]
		}
		return value
	case *registry.DecodedField:
		return value.Value
	case registry.DecodedField:
		return value.Value
	default:
		return value
	}
}

func cloneArrayTo[T any](_ T, array []interface{}) []T {
	cloned := make([]T, len(array))
	for i := range array {
		cloned[i] = array[i].(T)
	}
	return cloned
}

func (ev *Event) GetParam(name string, index int) (interface{}, bool) {
	if index >= len(ev.Raw.Fields) {
		return nil, false
	}
	value := ev.Raw.Fields[index]

	decoded := unwrap(value)
	switch maybeWrappedStill := decoded.(type) {
	case *registry.DecodedField:
		decoded = unwrap(maybeWrappedStill)
	case registry.DecodedField:
		decoded = unwrap(&maybeWrappedStill)
	}

	switch decoded := decoded.(type) {
	case []interface{}:
		if len(decoded) > 0 {
			switch element := decoded[0].(type) {
			case types.U8:
				bz := cloneArrayTo(element, decoded)
				addr, _ := codec.EncodeToHex(bz)
				return addr, true
			case byte:
				bz := cloneArrayTo(element, decoded)
				addr, _ := codec.EncodeToHex(bz)
				return addr, true

			default:
				logrus.WithField("type", fmt.Sprintf("%T", element)).Warn("unknown array type, could not decode unwrapped substrate event")
			}
		}
		return decoded, true
	case types.U64:
		return uint64(decoded), true
	case uint64:
		return decoded, true
	case types.U128:
		return decoded.String(), true
	default:
		bz, _ := json.MarshalIndent(decoded, "", "  ")
		fmt.Println(string(bz))
		logrus.WithField("type", fmt.Sprintf("%T", decoded)).Warn("unknown type, could not decode unwrapped substrate event")
		return nil, false
	}
}
