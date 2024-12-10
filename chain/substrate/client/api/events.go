package api

import (
	"fmt"
	"strings"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	xc "github.com/cordialsys/crosschain"
)

// An event is typically identified by something like "<module>.<event>", e.g. "Balances.Transfer"
type EventI interface {
	// Or may be called "pallet"
	GetModule() string
	// Or may just be the "name"
	GetEvent() string
	GetParam(name string, index int) (interface{}, bool)
}

type EventBind string

const BindFrom EventBind = "from"
const BindTo EventBind = "to"
const BindAmount EventBind = "amount"

type EventValueType string

const EventAddress EventValueType = "address"
const EventInteger EventValueType = "integer"

type EventAttributeDescriptor struct {
	// The attribute name, as shown in subscan extrinisic response
	Name string
	// The index in the attribute/param list that this attribute will appear
	// This is needed for the subquery indexer.
	Index int
	// Which attribute in a transfer this should bind to
	Bind EventBind
	// How to parse the type
	Type EventValueType
}

type EventDescriptor struct {
	Module     string
	Event      string
	Attributes []*EventAttributeDescriptor
}

var SupportedEvents = []EventDescriptor{
	{
		Module: "balances",
		Event:  "transfer",
		Attributes: []*EventAttributeDescriptor{
			{
				Name:  "from",
				Index: 0,
				Bind:  BindFrom,
				Type:  EventAddress,
			},
			{
				Name:  "to",
				Index: 1,
				Bind:  BindTo,
				Type:  EventAddress,
			},
			{
				Name:  "amount",
				Index: 2,
				Bind:  BindAmount,
				Type:  EventInteger,
			},
		},
	},
}

type eventHandleS string

var supportedEventMap = map[eventHandleS]EventDescriptor{}

func eventHandle(module, event string) eventHandleS {
	return eventHandleS(strings.ToLower(module) + "." + strings.ToLower(event))
}
func init() {
	for _, ev := range SupportedEvents {
		supportedEventMap[eventHandle(ev.Module, ev.Event)] = ev
	}
}

func ParseEvents(ab xc.AddressBuilder, chain xc.NativeAsset, events []EventI) (sources []*xc.LegacyTxInfoEndpoint, destinations []*xc.LegacyTxInfoEndpoint, err error) {
	for _, ev := range events {
		handle := eventHandle(ev.GetModule(), ev.GetEvent())
		desc, ok := supportedEventMap[handle]
		if !ok {
			continue
		}
		var from, to xc.Address
		var amount xc.AmountBlockchain
		for _, attr := range desc.Attributes {
			param, ok := ev.GetParam(attr.Name, attr.Index)
			if !ok {
				err = fmt.Errorf("substrate event %s did not contain expected param %s at index %d", handle, attr.Name, attr.Index)
				return nil, nil, err
			}
			switch attr.Type {
			case EventAddress:
				addr, ok := param.(string)
				if !ok {
					err = fmt.Errorf("substrate event %s attribute %s expected type %T but got %T", handle, attr.Name, addr, param)
					return nil, nil, err
				}
				var xcAddr xc.Address
				if strings.HasPrefix(addr, "0x") {
					addrBz, err := codec.HexDecodeString(addr)
					if err != nil {
						err = fmt.Errorf("substrate event %s attribute %s has invalid hex: %v", handle, attr.Name, addr)
						return nil, nil, err
					}
					xcAddr, _ = ab.GetAddressFromPublicKey(addrBz)
				} else {
					xcAddr = xc.Address(addr)
				}
				switch attr.Bind {
				case BindFrom:
					from = xcAddr
				case BindTo:
					to = xcAddr
				default:
					return nil, nil, fmt.Errorf("substrate event %s attribute %s has invalid bind configured: %s", handle, attr.Name, attr.Bind)
				}
			case EventInteger:
				asString := fmt.Sprint(param)
				switch attr.Bind {
				case BindAmount:
					amount = xc.NewAmountBlockchainFromStr(asString)
				default:
					return nil, nil, fmt.Errorf("substrate event %s attribute %s has invalid bind configured: %s", handle, attr.Name, attr.Bind)
				}
			}
		}

		if from != "" {
			sources = append(sources, &xc.LegacyTxInfoEndpoint{
				Address:     from,
				NativeAsset: chain,
				Amount:      amount,
			})
		}
		if to != "" {
			destinations = append(destinations, &xc.LegacyTxInfoEndpoint{
				Address:     to,
				NativeAsset: chain,
				Amount:      amount,
			})
		}
	}
	return
}
