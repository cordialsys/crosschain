package tron

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	xc "github.com/cordialsys/crosschain"
	httpclient "github.com/cordialsys/crosschain/chain/tron/http_client"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/ethereum/go-ethereum/common"
	evmtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/okx/go-wallet-sdk/crypto/base58"
)

func compareTopics(topics1 []httpclient.Bytes, topics2 []common.Hash) bool {
	equal := true
	for i, topic := range topics1 {
		if !bytes.Equal(topic, topics2[i][:]) {
			equal = false
			break
		}
	}
	return equal
}

func deserialiseTransactionEvents(log []*httpclient.Log, evmTx *evmtypes.Receipt) ([]*xclient.LegacyTxInfoEndpoint, []*xclient.LegacyTxInfoEndpoint) {
	sources := make([]*xclient.LegacyTxInfoEndpoint, 0)
	destinations := make([]*xclient.LegacyTxInfoEndpoint, 0)

	for _, event := range log {
		source := new(xclient.LegacyTxInfoEndpoint)
		destination := new(xclient.LegacyTxInfoEndpoint)
		source.NativeAsset = xc.TRX
		destination.NativeAsset = xc.TRX
		if len(event.Topics) < 3 {
			continue
		}

		// The addresses in the TVM omits the prefix 0x41, so we add it here to allow us to parse the addresses
		eventContractB58 := base58.CheckEncode(event.Address, 0x41)
		eventSourceB58 := base58.CheckEncode(event.Topics[1][12:], 0x41)      // Remove padding
		eventDestinationB58 := base58.CheckEncode(event.Topics[2][12:], 0x41) // Remove padding
		eventMethodBz := event.Topics[0]

		eventValue := new(big.Int)
		eventValue.SetString(hex.EncodeToString(event.Data), 16) // event value is returned as a padded big int hex

		if hex.EncodeToString(eventMethodBz) != strings.TrimPrefix(TRANSFER_EVENT_HASH_HEX, "0x") {
			continue
		}

		source.ContractAddress = xc.ContractAddress(eventContractB58)
		destination.ContractAddress = xc.ContractAddress(eventContractB58)

		source.Address = xc.Address(eventSourceB58)
		source.Amount = xc.NewAmountBlockchainFromUint64(eventValue.Uint64())
		destination.Address = xc.Address(eventDestinationB58)
		destination.Amount = xc.NewAmountBlockchainFromUint64(eventValue.Uint64())

		for _, log := range evmTx.Logs {
			if compareTopics(event.Topics, log.Topics) {
				// Use the index from the evm log, as this is a block offset and is strictly more useful
				// then using a transaction index.  This is because tron transactions always have a single log,
				// meaning the index would always be 0.
				index := log.Index
				source.Event = xclient.NewEventFromIndex(uint64(index), xclient.MovementVariantToken)
				destination.Event = xclient.NewEventFromIndex(uint64(index), xclient.MovementVariantToken)
				break
			}
		}

		sources = append(sources, source)
		destinations = append(destinations, destination)
	}

	return sources, destinations
}

func deserialiseNativeTransfer(tx *httpclient.GetTransactionIDResponse) (xc.Address, xc.Address, xc.AmountBlockchain, error) {
	if len(tx.RawData.Contract) != 1 {
		return "", "", xc.AmountBlockchain{}, fmt.Errorf("unsupported transaction")
	}

	contract := tx.RawData.Contract[0]

	if contract.Type != "TransferContract" {
		return "", "", xc.AmountBlockchain{}, fmt.Errorf("unsupported transaction")
	}
	transferContract, err := contract.AsTransferContract()
	if err != nil {
		return "", "", xc.AmountBlockchain{}, fmt.Errorf("invalid transfer-contract: %v", err)
	}

	from := xc.Address(transferContract.Owner)
	to := xc.Address(transferContract.To)
	amount := transferContract.Amount

	return from, to, xc.NewAmountBlockchainFromUint64(uint64(amount)), nil
}
