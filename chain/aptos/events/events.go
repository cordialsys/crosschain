package events

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coming-chat/go-aptos/aptostypes"
	xc "github.com/cordialsys/crosschain"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/sirupsen/logrus"
)

func reserializeJson(obj any, target any) error {
	bz, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return json.Unmarshal(bz, target)
}

// Transform "0x1::coin::CoinStore<x>" -> x
func parseContractAddress(typeString string) string {
	typeString = strings.Replace(typeString, "0x1::coin::CoinStore<", "", 1)
	typeString = strings.Replace(typeString, ">", "", 1)
	return typeString
}

type CoinDepositEvent struct {
	Amount string `json:"amount"`
}
type CoinWithdrawEvent = CoinDepositEvent // same structure

// Same for both withdraw and deposit
type FungibleAssetEvent struct {
	Amount string `json:"amount"`
	Store  string `json:"store"`
}

func ParseEvents(tx *aptostypes.Transaction, txHash xc.TxHash) (sources []*txinfo.LegacyTxInfoEndpoint, destinations []*txinfo.LegacyTxInfoEndpoint, err error) {
	log := logrus.WithField("txhash", txHash)

	changes := []*ParsedChange{}
	for _, ch := range tx.Changes {
		parsed, err := ParseChange(&ch)
		if err != nil {
			log.WithError(err).Error("could not parse aptos change")
			continue
		}
		changes = append(changes, parsed)
	}

	for i, event := range tx.Events {
		switch event.Type {
		case "0x1::coin::DepositEvent", "0x1::coin::WithdrawEvent":
			withdraw := &CoinWithdrawEvent{}
			err := reserializeJson(event.Data, withdraw)
			if err != nil {
				log.WithError(err).Error("could not deserialize aptos coin event")
				continue
			}

			// Need to join the event with the corresponding "change" in order
			// to figure out the asset_id/contract address.
			contract := ""
			for _, change := range changes {
				coinStore, ok := change.AsCoinStore()
				if ok {
					if event.Guid.AccountAddress == coinStore.DepositEvents.Guid.Id.AccountAddress &&
						event.Guid.CreationNumber == coinStore.DepositEvents.Guid.Id.CreationNumber {
						contract = parseContractAddress(change.Inner.Type)
					} else if event.Guid.AccountAddress == coinStore.WithdrawEvents.Guid.Id.AccountAddress &&
						event.Guid.CreationNumber == coinStore.WithdrawEvents.Guid.Id.CreationNumber {
						contract = parseContractAddress(change.Inner.Type)
					}
				}
				if contract != "" {
					break
				}
			}
			if contract == "" {
				log.WithField("event", event.Type).Warn("could not find contract for coin event")
				continue
			}

			endpoint := &txinfo.LegacyTxInfoEndpoint{
				ContractAddress: xc.ContractAddress(contract),
				Address:         xc.Address(event.Guid.AccountAddress),
				Amount:          xc.NewAmountBlockchainFromStr(withdraw.Amount),
				Event:           txinfo.NewEventFromIndex(uint64(i), txinfo.MovementVariantNative),
			}
			if event.Type == "0x1::coin::WithdrawEvent" {
				fmt.Println("adding destination", endpoint.Address)
				sources = append(sources, endpoint)
			} else {
				fmt.Println("adding source", endpoint.Address)
				destinations = append(destinations, endpoint)
			}
		case "0x1::fungible_asset::Deposit", "0x1::fungible_asset::Withdraw":
			withdraw := &FungibleAssetEvent{}
			err := reserializeJson(event.Data, withdraw)
			if err != nil {
				log.WithError(err).Error("could not deserialize fungible asset event")
				continue
			}
			// in order to figure out what addresses these are for, we need to match them to the object core change
			// which has a matching object address.
			var address xc.Address
			for _, change := range changes {
				if change.Change.Address == withdraw.Store {
					objectCore, ok := change.AsObjectCore()
					if ok {
						// we have a match.
						address = xc.Address(objectCore.Owner)
						break
					}
				}
			}
			if address == "" {
				log.WithFields(logrus.Fields{
					"event":         event.Type,
					"store_address": withdraw.Store,
				}).Warn("could not find address for fungible asset event")
				continue
			}

			// The event also does not include the contract address, so we need to make yet
			// another join to figure that out.
			var contract xc.ContractAddress
			for _, change := range changes {
				if change.Change.Address == withdraw.Store {
					fungibleStore, ok := change.AsFungibleStore()
					if ok {
						contract = xc.ContractAddress(fungibleStore.Metadata.Inner)
					}
				}
			}
			if contract == "" {
				log.WithField("event", event.Type).Warn("could not find contract for fungible asset event")
				continue
			}
			endpoint := &txinfo.LegacyTxInfoEndpoint{
				ContractAddress: xc.ContractAddress(contract),
				Address:         address,
				Amount:          xc.NewAmountBlockchainFromStr(withdraw.Amount),
				Event:           txinfo.NewEventFromIndex(uint64(i), txinfo.MovementVariantNative),
			}
			if event.Type == "0x1::fungible_asset::Withdraw" {
				sources = append(sources, endpoint)
			} else {
				destinations = append(destinations, endpoint)
			}
		default:
			// skip / unknown.
			log.WithField("event", event.Type).Debug("unknown event")
		}
	}
	for _, source := range sources {
		source.NativeAsset = xc.APTOS
	}
	for _, destination := range destinations {
		destination.NativeAsset = xc.APTOS
	}

	return sources, destinations, nil
}
