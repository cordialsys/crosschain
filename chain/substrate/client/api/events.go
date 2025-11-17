package api

import (
	"fmt"
	"strings"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	xc "github.com/cordialsys/crosschain"
	xcerrors "github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/sirupsen/logrus"
)

// An event is typically identified by something like "<module>.<event-id>", e.g. "Balances.Transfer"
type EventI interface {
	// Or may be called "pallet"
	GetModule() string
	// Or may just be the "name" or id
	GetId() string
	GetParam(name string, index int) (interface{}, bool)
	GetEventDescriptor() (*txinfo.Event, bool)
}

type EventBind string

const BindFrom EventBind = "from"
const BindTo EventBind = "to"
const BindAmount EventBind = "amount"
const BindValidator EventBind = "validator"

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
	Stake      bool
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
	{
		Module: "NominationPools",
		Event:  "Withdrawn",
		Attributes: []*EventAttributeDescriptor{
			{
				Name:  "member",
				Index: 0,
				Bind:  BindTo,
				Type:  EventAddress,
			},
			{
				Name:  "balance",
				Index: 2,
				Bind:  BindAmount,
				Type:  EventInteger,
			},
		},
	},
}

var SupportedStakingEvents = []EventDescriptor{
	{
		Module: "SubtensorModule",
		Event:  "StakeAdded",
		Stake:  true,
		Attributes: []*EventAttributeDescriptor{
			{
				Name:  "from",
				Index: 1,
				Bind:  BindValidator,
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
	{
		Module: "SubtensorModule",
		Event:  "StakeRemoved",
		Stake:  false,
		Attributes: []*EventAttributeDescriptor{
			{
				Name:  "from",
				Index: 0,
				Bind:  BindFrom,
				Type:  EventAddress,
			},
			{
				Name:  "validator",
				Index: 1,
				Bind:  BindValidator,
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
	// Nomination Pools events
	{
		Module: "NominationPools",
		Event:  "Bonded",
		Stake:  true,
		Attributes: []*EventAttributeDescriptor{
			{
				Name:  "member",
				Index: 0,
				Bind:  BindFrom,
				Type:  EventAddress,
			},
			{
				Name:  "pool_id",
				Index: 1,
				Bind:  BindValidator,
				Type:  EventInteger,
			},
			{
				Name:  "bonded",
				Index: 2,
				Bind:  BindAmount,
				Type:  EventInteger,
			},
		},
	},
	{
		Module: "NominationPools",
		Event:  "Unbonded",
		Stake:  false,
		Attributes: []*EventAttributeDescriptor{
			{
				Name:  "member",
				Index: 0,
				Bind:  BindFrom,
				Type:  EventAddress,
			},
			{
				Name:  "pool_id",
				Index: 1,
				Bind:  BindValidator,
				Type:  EventInteger,
			},
			{
				Name:  "balance",
				Index: 2,
				Bind:  BindAmount,
				Type:  EventInteger,
			},
		},
	},
}

type eventHandleS string

var supportedEventMap = map[eventHandleS]EventDescriptor{}
var supportedStakingEventMap = map[eventHandleS]EventDescriptor{}

func eventHandle(module, event string) eventHandleS {
	return eventHandleS(strings.ToLower(module) + "." + strings.ToLower(event))
}
func init() {
	for _, ev := range SupportedEvents {
		supportedEventMap[eventHandle(ev.Module, ev.Event)] = ev
	}
	for _, ev := range SupportedStakingEvents {
		supportedStakingEventMap[eventHandle(ev.Module, ev.Event)] = ev
	}
}

func ParseAddress(ab xc.AddressBuilder, addr string) (xc.Address, error) {
	var xcAddr xc.Address
	if strings.HasPrefix(addr, "0x") {
		addrBz, err := codec.HexDecodeString(addr)
		if err != nil {
			err = fmt.Errorf("substrate address %s has invalid hex: %v", addr, err)
			return "", err
		}
		xcAddr, err = ab.GetAddressFromPublicKey(addrBz)
		return xcAddr, err
	} else {
		xcAddr = xc.Address(addr)
	}
	return xcAddr, nil
}

func find(events []EventI, module, event string) (EventI, bool) {
	for _, ev := range events {
		if strings.EqualFold(ev.GetModule(), module) && strings.EqualFold(ev.GetId(), event) {
			return ev, true
		}
	}
	return nil, false
}

func ParseFailed(events []EventI) (string, bool) {
	ev, ok := find(events, "systems", "extrinsicfailure")
	if ok {
		reason, ok := ev.GetParam("", 0)
		if ok {
			asString, ok := reason.(string)
			if ok {
				return xcerrors.TransactionFailuref("%s", asString).Error(), true
			} else {
				logrus.WithField("type", fmt.Sprintf("%T", reason)).Warn("did not expect type for failure")
			}
		}
		return xcerrors.TransactionFailuref("unknown").Error(), true
	}
	ev, ok = find(events, "System", "ExtrinsicFailed")
	if ok {
		param, ok := ev.GetParam("dispatch_error", 0)
		if ok {
			module, ok := param.(map[string]interface{})["Module"].(map[string]interface{})
			if ok {
				errCode, ok := module["error"].(string)
				if ok {
					return xcerrors.TransactionFailuref("%s", errCode).Error(), true
				}
			}

		}

		// too difficult to decode further to sus out an error code or message
		return xcerrors.TransactionFailuref("unable to decode reason").Error(), true
	}
	return "", false
}

func ParseFee(ab xc.AddressBuilder, events []EventI) (xc.Address, xc.AmountBlockchain, bool, error) {
	ev, ok := find(events, "TransactionPayment", "TransactionFeePaid")
	if ok {
		who, ok := ev.GetParam("who", 0)
		if !ok {
			return "", xc.AmountBlockchain{}, false, fmt.Errorf("TransactionPayment.TransactionFeePaid did not have 0 param")
		}
		whoString, ok := who.(string)
		if !ok {
			return "", xc.AmountBlockchain{}, false, fmt.Errorf("TransactionPayment.TransactionFeePaid 0 param unexpected type: %T", who)
		}
		addr, err := ParseAddress(ab, whoString)
		if err != nil {
			return "", xc.AmountBlockchain{}, false, fmt.Errorf("TransactionPayment.TransactionFeePaid who invalid address: %v", err)
		}
		amountRaw, ok := ev.GetParam("actual_fee", 1)
		if !ok {
			amountRaw, ok = ev.GetParam("actualFee", 1)
			if !ok {
				return "", xc.AmountBlockchain{}, false, fmt.Errorf("TransactionPayment.TransactionFeePaid amount missing")
			}
		}
		amount := xc.NewAmountBlockchainFromStr(fmt.Sprint(amountRaw))
		return addr, amount, true, nil
	}
	// no fee detected
	return "", xc.AmountBlockchain{}, false, nil
}
func ParseEvents(ab xc.AddressBuilder, chain xc.NativeAsset, events []EventI) (sources []*txinfo.LegacyTxInfoEndpoint, destinations []*txinfo.LegacyTxInfoEndpoint, err error) {
	for _, ev := range events {
		handle := eventHandle(ev.GetModule(), ev.GetId())
		desc, ok := supportedEventMap[handle]
		if !ok {
			continue
		}
		var from, to xc.Address
		var amount xc.AmountBlockchain
		var eventDescriptor *txinfo.Event
		if desc, ok := ev.GetEventDescriptor(); ok {
			eventDescriptor = desc
		}
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
				xcAddr, err := ParseAddress(ab, addr)
				if !ok {
					return nil, nil, err
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
			sources = append(sources, &txinfo.LegacyTxInfoEndpoint{
				Address:     from,
				NativeAsset: chain,
				Amount:      amount,
				Event:       eventDescriptor,
			})
		}
		if to != "" {
			destinations = append(destinations, &txinfo.LegacyTxInfoEndpoint{
				Address:     to,
				NativeAsset: chain,
				Amount:      amount,
				Event:       eventDescriptor,
			})
		}
	}
	return
}

func ParseStakingEvents(ab xc.AddressBuilder, chain xc.NativeAsset, events []EventI) (stakes []*txinfo.Stake, unstakes []*txinfo.Unstake, err error) {
	for _, ev := range events {
		handle := eventHandle(ev.GetModule(), ev.GetId())
		desc, ok := supportedStakingEventMap[handle]
		if !ok {
			continue
		}
		var validator xc.Address
		var from xc.Address
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
				xcAddr, err := ParseAddress(ab, addr)
				if !ok {
					return nil, nil, err
				}
				switch attr.Bind {
				case BindFrom:
					from = xcAddr
				case BindValidator:
					validator = xcAddr
				default:
					return nil, nil, fmt.Errorf("substrate event %s attribute %s has invalid bind configured: %s", handle, attr.Name, attr.Bind)
				}

			case EventInteger:
				asString := fmt.Sprint(param)
				switch attr.Bind {
				case BindAmount:
					amount = xc.NewAmountBlockchainFromStr(asString)
				case BindValidator:
					// For nomination pools, validator is a pool ID (integer)
					validator = xc.Address(asString)
				default:
					return nil, nil, fmt.Errorf("substrate event %s attribute %s has invalid bind configured: %s", handle, attr.Name, attr.Bind)
				}
			}
		}

		if desc.Stake {
			stakes = append(stakes, &txinfo.Stake{
				Validator: string(validator),
				Balance:   amount,
				Address:   string(from),
			})
		} else {
			unstakes = append(unstakes, &txinfo.Unstake{
				Validator: string(validator),
				Balance:   amount,
				Address:   string(from),
			})
		}

	}
	return
}
