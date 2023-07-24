package aptos

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/coming-chat/go-aptos/aptostypes"
	transactionbuilder "github.com/coming-chat/go-aptos/transaction_builder"
	"github.com/coming-chat/lcs"
	xc "github.com/jumpcrypto/crosschain"
	"github.com/sirupsen/logrus"
)

func mustDecodeHex(h string) []byte {
	h = strings.Replace(h, "0x", "", 1)
	bz, err := hex.DecodeString(h)
	if err != nil {
		panic(err)
	}
	return bz
}

func destinationsFromTxPayload(payload transactionbuilder.TransactionPayload) []*xc.TxInfoEndpoint {
	switch payload := payload.(type) {
	case *transactionbuilder.TransactionPayloadEntryFunction:
		if len(payload.Args) > 0 {
			amount := uint64(0)
			if len(payload.Args) > 1 {
				err := lcs.Unmarshal(payload.Args[1], &amount)
				if err != nil {
					logrus.Errorf("could not unmarshal amount: %v\n", err)
					return []*xc.TxInfoEndpoint{}
				}
			}
			to_addr := payload.Args[0]
			return []*xc.TxInfoEndpoint{
				{
					Address: xc.Address(to_addr),
					Amount:  xc.NewAmountBlockchainFromUint64(amount),
				},
			}
		}
	case *aptostypes.Payload:
		switch payload.Function {
		case "0x1::aptos_account::batch_transfer_coins":
			fmt.Println("0", payload.Arguments[0])
			fmt.Println("1", payload.Arguments[1])
			addresses := payload.Arguments[0].([]interface{})
			amounts := payload.Arguments[1].([]interface{})
			destinations := []*xc.TxInfoEndpoint{}
			for i := 0; i < len(addresses) && i < len(amounts); i++ {
				amountStr := amounts[i].(string)
				destinations = append(destinations, &xc.TxInfoEndpoint{
					Address: xc.Address(addresses[i].(string)),
					Amount:  xc.NewAmountBlockchainFromStr(amountStr),
				})
			}
			return destinations
		case "0x1::aptos_account::transfer":
			amount := xc.NewAmountBlockchainFromUint64(0)
			if len(payload.Arguments) > 1 {
				amount = xc.NewAmountBlockchainFromStr(payload.Arguments[1].(string))
			}
			to_addr := payload.Arguments[0].(string)
			return []*xc.TxInfoEndpoint{
				{
					Address: xc.Address(to_addr),
					Amount:  amount,
				},
			}
		default:
			logrus.Errorf("unrecognized payload function: %s\n", payload.Function)
		}
	default:
		logrus.Errorf("unrecognized payload type: %T\n", payload)
	}
	return []*xc.TxInfoEndpoint{}
}
