package client

import (
	"encoding/base64"

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
			var transferEvent TransferEvent
			for _, attr := range event.Attributes {
				if attr.Key == "recipient" {
					transferEvent.Recipient = attr.Value
				}
				if attr.Key == "sender" {
					transferEvent.Sender = attr.Value
				}
				if attr.Key == "amount" {
					coin, _ := types.ParseCoinNormalized(attr.Value)
					transferEvent.Amount = xc.AmountBlockchain(*coin.Amount.BigInt())
					transferEvent.Contract = coin.Denom
				}
			}
			// if transferEvent.Sender != sender {
			// 	// drop transfers not originating from the sender, otherwise we get spammy events
			// 	// relating to inter-module cosmos transfers
			// 	continue
			// }
			parseEvents.Transfers = append(parseEvents.Transfers, transferEvent)
		}
		// parse withdraw rewards event
		if event.Type == "withdraw_rewards" {
			var withdrawRewardsEvent WithdrawRewardsEvent
			for _, attr := range event.Attributes {
				if attr.Key == "validator" {
					withdrawRewardsEvent.Validator = attr.Value
				}
				if attr.Key == "delegator" {
					withdrawRewardsEvent.Delegator = attr.Value
				}
				if attr.Key == "amount" {
					coin, _ := types.ParseCoinNormalized(attr.Value)
					withdrawRewardsEvent.Amount = xc.AmountBlockchain(*coin.Amount.BigInt())
					withdrawRewardsEvent.Contract = coin.Denom
				}
			}
			parseEvents.Withdraws = append(parseEvents.Withdraws, withdrawRewardsEvent)
		}
		// parse delegate event
		if event.Type == "delegate" {
			var delegateEvent DelegateEvent
			for _, attr := range event.Attributes {
				if attr.Key == "validator" {
					delegateEvent.Validator = attr.Value
				}
				if attr.Key == "delegator" {
					delegateEvent.Delegator = attr.Value
				}
				if attr.Key == "amount" {
					coin, _ := types.ParseCoinNormalized(attr.Value)
					delegateEvent.Amount = xc.AmountBlockchain(*coin.Amount.BigInt())
					delegateEvent.Contract = coin.Denom
				}
			}
			parseEvents.Delegates = append(parseEvents.Delegates, delegateEvent)
		}
		// parse unbond event
		if event.Type == "unbond" {
			var unbondEvent UnbondEvent
			for _, attr := range event.Attributes {
				if attr.Key == "validator" {
					unbondEvent.Validator = attr.Value
				}
				if attr.Key == "delegator" {
					unbondEvent.Delegator = attr.Value
				}
				if attr.Key == "amount" {
					coin, _ := types.ParseCoinNormalized(attr.Value)
					unbondEvent.Amount = xc.AmountBlockchain(*coin.Amount.BigInt())
					unbondEvent.Contract = coin.Denom
				}
			}
			parseEvents.Unbonds = append(parseEvents.Unbonds, unbondEvent)
		}
		// parse wasm CW20 transfer event
		if event.Type == "wasm" {
			var action string
			for _, attr := range event.Attributes {
				if attr.Key == "action" {
					action = attr.Value
					break
				}
			}
			if action == "transfer" {
				var transferEvent TransferEvent
				for _, attr := range event.Attributes {
					if attr.Key == "to" {
						transferEvent.Recipient = attr.Value
					}
					if attr.Key == "from" {
						transferEvent.Sender = attr.Value
					}
					if attr.Key == "amount" {
						transferEvent.Amount = xc.NewAmountBlockchainFromStr(attr.Value)
					}
					if attr.Key == "_contract_address" || attr.Key == "contract_address" {
						transferEvent.Contract = attr.Value
					}
				}
				parseEvents.Transfers = append(parseEvents.Transfers, transferEvent)
			}
		}
	}
	return parseEvents
}
