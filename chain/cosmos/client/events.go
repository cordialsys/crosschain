package client

import (
	"encoding/base64"
	"strings"

	comettypes "github.com/cometbft/cometbft/abci/types"
	xc "github.com/cordialsys/crosschain"
	"github.com/cosmos/cosmos-sdk/types"
)

type TransferEvent struct {
	Recipient string
	Sender    string
	Amount    xc.AmountBlockchain
	Contract  string
}
type WithdrawRewardsEvent struct {
	Validator string
	Delegator string
	Amount    xc.AmountBlockchain
	Contract  string
}

type DelegateEvent struct {
	Validator string
	Delegator string
	Amount    xc.AmountBlockchain
	Contract  string
}

type UnbondEvent struct {
	Validator string
	Delegator string
	Amount    xc.AmountBlockchain
	Contract  string
}

type ParsedEvents struct {
	Transfers []TransferEvent
	// Every withdraw event also has a transfer event; so far we can ignore these
	Withdraws []WithdrawRewardsEvent
	Delegates []DelegateEvent
	Unbonds   []UnbondEvent
}

func DecodeEventAttributes(attrs []comettypes.EventAttribute) {
	// For some strange reason, event attributes are base64 encoded on some cosmos chains, but not all.
	// E.g. not base64 encoded on Injective, but they are on Terra.
	for i := range attrs {
		key, err1 := base64.StdEncoding.DecodeString(attrs[i].Key)
		value, err2 := base64.StdEncoding.DecodeString(attrs[i].Value)
		if err1 == nil && err2 == nil {
			attrs[i].Key = string(key)
			attrs[i].Value = string(value)
		}
	}
}

func getEvent(events comettypes.Event, key string) (string, bool) {
	for _, ev := range events.Attributes {
		if ev.Key == key {
			return ev.Value, true
		}
	}
	return "", false
}

func getEventOrZero(events comettypes.Event, key string) string {
	v, _ := getEvent(events, key)
	return v
}

func ParseEvents(events []comettypes.Event) ParsedEvents {
	parseEvents := ParsedEvents{}
	var sender string
	for _, event := range events {
		if event.Type == "message" {
			for _, attr := range event.Attributes {
				if attr.Key == "sender" {
					sender = attr.Value
					break
				}
			}
		}
		if sender != "" {
			break
		}
	}
	foundMsgEvent := false
	for _, event := range events {
		DecodeEventAttributes(event.Attributes)

		if event.Type == "message" {
			foundMsgEvent = true
		}

		if !foundMsgEvent {
			// all events before the message-related events are fee related, and we can ignore them,
			// or else we'll double report fees as transfers.
			continue
		}
		if event.Type == "transfer" {
			var amounts []TransferEvent
			amountValue, ok := getEvent(event, "amount")
			if ok {
				// E.g.
				// https://finder.terra.money/classic/tx/5dc2034edcfdb74a5d33d73b9ccd9cce034d7950e5691c2be5ddd93e6091cfff
				amountParts := strings.Split(amountValue, ",")
				for _, amountValue := range amountParts {
					coin, _ := types.ParseCoinNormalized(amountValue)
					amounts = append(amounts, TransferEvent{
						Amount:    xc.AmountBlockchain(*coin.Amount.BigInt()),
						Contract:  coin.Denom,
						Recipient: getEventOrZero(event, "recipient"),
						Sender:    getEventOrZero(event, "sender"),
					})
				}
			}
			parseEvents.Transfers = amounts
		}
		if event.Type == "withdraw_rewards" {
			var amounts []WithdrawRewardsEvent
			amountValue, ok := getEvent(event, "amount")
			if ok {
				amountParts := strings.Split(amountValue, ",")
				for _, amountValue := range amountParts {
					coin, _ := types.ParseCoinNormalized(amountValue)
					amounts = append(amounts, WithdrawRewardsEvent{
						Amount:    xc.AmountBlockchain(*coin.Amount.BigInt()),
						Contract:  coin.Denom,
						Validator: getEventOrZero(event, "validator"),
						Delegator: getEventOrZero(event, "delegator"),
					})
				}
			}
			parseEvents.Withdraws = amounts
		}
		if event.Type == "delegate" {
			var amounts []DelegateEvent
			amountValue, ok := getEvent(event, "amount")
			if ok {
				amountParts := strings.Split(amountValue, ",")
				for _, amountValue := range amountParts {
					coin, _ := types.ParseCoinNormalized(amountValue)
					amounts = append(amounts, DelegateEvent{
						Amount:    xc.AmountBlockchain(*coin.Amount.BigInt()),
						Contract:  coin.Denom,
						Validator: getEventOrZero(event, "validator"),
						Delegator: getEventOrZero(event, "delegator"),
					})
				}
			}
			parseEvents.Delegates = amounts
		}
		if event.Type == "unbond" {
			var amounts []UnbondEvent
			amountValue, ok := getEvent(event, "amount")
			if ok {
				amountParts := strings.Split(amountValue, ",")
				for _, amountValue := range amountParts {
					coin, _ := types.ParseCoinNormalized(amountValue)
					amounts = append(amounts, UnbondEvent{
						Amount:    xc.AmountBlockchain(*coin.Amount.BigInt()),
						Contract:  coin.Denom,
						Validator: getEventOrZero(event, "validator"),
						Delegator: getEventOrZero(event, "delegator"),
					})
				}
			}
			parseEvents.Unbonds = amounts
		}

		if event.Type == "wasm" {
			var amounts []TransferEvent
			action, _ := getEvent(event, "action")
			amountValue, ok := getEvent(event, "amount")

			if ok && action == "transfer" {
				amountParts := strings.Split(amountValue, ",")
				for _, amountValue := range amountParts {
					tf := TransferEvent{
						Amount:    xc.NewAmountBlockchainFromStr(amountValue),
						Contract:  getEventOrZero(event, "contract_address"),
						Recipient: getEventOrZero(event, "to"),
						Sender:    getEventOrZero(event, "from"),
					}
					if tf.Contract == "" {
						tf.Contract = getEventOrZero(event, "_contract_address")
					}
					amounts = append(amounts, tf)
				}
			}
			parseEvents.Transfers = append(parseEvents.Transfers, amounts...)
		}
	}
	return parseEvents
}
