package client

import (
	"encoding/base64"
	"strings"

	comettypes "github.com/cometbft/cometbft/abci/types"
	xc "github.com/cordialsys/crosschain"
	"github.com/cosmos/cosmos-sdk/types"
)

type EventIndex struct {
	Index int
}

type TransferEvent struct {
	EventIndex
	Recipient string
	Sender    string
	Amount    xc.AmountBlockchain
	Contract  string
}
type WithdrawRewardsEvent struct {
	EventIndex
	Validator string
	Delegator string
	Amount    xc.AmountBlockchain
	Contract  string
}

type DelegateEvent struct {
	EventIndex
	Validator string
	Delegator string
	Amount    xc.AmountBlockchain
	Contract  string
}

type UnbondEvent struct {
	EventIndex
	Validator string
	Delegator string
	Amount    xc.AmountBlockchain
	Contract  string
}

type Fee struct {
	EventIndex
	Amount xc.AmountBlockchain
	// Contract address of asset (may be chain coin)
	Contract string
	// Address of who paid
	Payer string
}

type ParsedEvents struct {
	ParsedMsgEvents
	ParsedTxEvents
}

type ParsedMsgEvents struct {
	Transfers []TransferEvent
	// Every withdraw event also has a transfer event; so far we can ignore these
	Withdraws []WithdrawRewardsEvent
	Delegates []DelegateEvent
	Unbonds   []UnbondEvent
}

type ParsedTxEvents struct {
	Fees []Fee
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
	for _, event := range events {
		// cosmos-sdk natively base64 encodes everything so we unwrap that in place
		DecodeEventAttributes(event.Attributes)
	}

	return ParsedEvents{
		ParsedMsgEvents: parseMsgEvents(events),
		ParsedTxEvents:  parseTxEvents(events),
	}
}

func parseMsgEvents(events []comettypes.Event) ParsedMsgEvents {
	parseEvents := ParsedMsgEvents{}
	foundMsgEvent := false
	for i, event := range events {
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
						EventIndex: EventIndex{Index: i},
						Amount:     xc.AmountBlockchain(*coin.Amount.BigInt()),
						Contract:   coin.Denom,
						Recipient:  getEventOrZero(event, "recipient"),
						Sender:     getEventOrZero(event, "sender"),
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
						EventIndex: EventIndex{Index: i},
						Amount:     xc.AmountBlockchain(*coin.Amount.BigInt()),
						Contract:   coin.Denom,
						Validator:  getEventOrZero(event, "validator"),
						Delegator:  getEventOrZero(event, "delegator"),
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
						EventIndex: EventIndex{Index: i},
						Amount:     xc.AmountBlockchain(*coin.Amount.BigInt()),
						Contract:   coin.Denom,
						Validator:  getEventOrZero(event, "validator"),
						Delegator:  getEventOrZero(event, "delegator"),
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
						EventIndex: EventIndex{Index: i},
						Amount:     xc.AmountBlockchain(*coin.Amount.BigInt()),
						Contract:   coin.Denom,
						Validator:  getEventOrZero(event, "validator"),
						Delegator:  getEventOrZero(event, "delegator"),
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
						EventIndex: EventIndex{Index: i},
						Amount:     xc.NewAmountBlockchainFromStr(amountValue),
						Contract:   getEventOrZero(event, "contract_address"),
						Recipient:  getEventOrZero(event, "to"),
						Sender:     getEventOrZero(event, "from"),
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

func parseTxEvents(events []comettypes.Event) ParsedTxEvents {
	parseEvents := ParsedTxEvents{}

	for i, event := range events {

		if event.Type == "tx" {
			var fees []Fee
			feeAmount, ok := getEvent(event, "fee")
			feePayer, _ := getEvent(event, "fee_payer")
			if ok && feeAmount != "" {
				amountParts := strings.Split(feeAmount, ",")
				for _, amountValue := range amountParts {
					coin, _ := types.ParseCoinNormalized(amountValue)
					fee := Fee{
						EventIndex: EventIndex{Index: i},
						Amount:     xc.AmountBlockchain(*coin.Amount.BigInt()),
						Contract:   coin.Denom,
						Payer:      feePayer,
					}
					fees = append(fees, fee)
				}
			}
			parseEvents.Fees = fees
		}
	}
	return parseEvents
}
