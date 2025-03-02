package api_test

import (
	"encoding/json"
	"testing"

	"github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate/address"
	"github.com/cordialsys/crosschain/chain/substrate/client/api"
	"github.com/cordialsys/crosschain/chain/substrate/client/api/taostats"
	"github.com/stretchr/testify/require"
)

func TestTaoEvents(t *testing.T) {
	respRaw := `{
  "pagination": {
    "current_page": 1,
    "per_page": 50,
    "total_items": 4,
    "total_pages": 1,
    "next_page": null,
    "prev_page": null
  },
  "data": [
    {
      "id": "4947436-0036",
      "extrinsic_index": 14,
      "index": 36,
      "phase": "ApplyExtrinsic",
      "pallet": "System",
      "name": "ExtrinsicSuccess",
      "full_name": "System.ExtrinsicSuccess",
      "args": {
        "dispatchInfo": {
          "class": {
            "__kind": "Normal"
          },
          "paysFee": {
            "__kind": "No"
          },
          "weight": {
            "proofSize": "0",
            "refTime": "1181074000"
          }
        }
      },
      "block_number": 4947436,
      "extrinsic_id": "4947436-0014",
      "call_id": null,
      "timestamp": "2025-02-17T15:59:48Z"
    },
    {
      "id": "4947436-0035",
      "extrinsic_index": 14,
      "index": 35,
      "phase": "ApplyExtrinsic",
      "pallet": "TransactionPayment",
      "name": "TransactionFeePaid",
      "full_name": "TransactionPayment.TransactionFeePaid",
      "args": {
        "actualFee": "1234",
        "tip": "0",
        "who": "0x32aca24932cb5827a56aa1a899ef681012102c53d34a9377f1c5c59bb72ce042"
      },
      "block_number": 4947436,
      "extrinsic_id": "4947436-0014",
      "call_id": "4947436-0014",
      "timestamp": "2025-02-17T15:59:48Z"
    },
    {
      "id": "4947436-0034",
      "extrinsic_index": 14,
      "index": 34,
      "phase": "ApplyExtrinsic",
      "pallet": "SubtensorModule",
      "name": "StakeAdded",
      "full_name": "SubtensorModule.StakeAdded",
      "args": [
        "0x32aca24932cb5827a56aa1a899ef681012102c53d34a9377f1c5c59bb72ce042",
        "0xfa855ae0c00667324315f7a72c60361dfe7b749a4935e238413c72d921cc4572",
        "3250000",
        "28669204",
        34
      ],
      "block_number": 4947436,
      "extrinsic_id": "4947436-0014",
      "call_id": "4947436-0014",
      "timestamp": "2025-02-17T15:59:48Z"
    },
    {
      "id": "4947436-0033",
      "extrinsic_index": 14,
      "index": 33,
      "phase": "ApplyExtrinsic",
      "pallet": "Balances",
      "name": "Withdraw",
      "full_name": "Balances.Withdraw",
      "args": {
        "amount": "3300000",
        "who": "0x32aca24932cb5827a56aa1a899ef681012102c53d34a9377f1c5c59bb72ce042"
      },
      "block_number": 4947436,
      "extrinsic_id": "4947436-0014",
      "call_id": "4947436-0014",
      "timestamp": "2025-02-17T15:59:48Z"
    }
  ]
}`
	resp := &taostats.GetEventsResponse{}
	err := json.Unmarshal([]byte(respRaw), resp)
	require.NoError(t, err)

	chain := crosschain.NewChainConfig(crosschain.TAO).WithChainPrefix("42")
	addressBuilder, err := address.NewAddressBuilder(chain)
	require.NoError(t, err)

	var eventsI = []api.EventI{}
	for _, ev := range resp.Data {
		eventsI = append(eventsI, ev)
	}
	sources, dests, err := api.ParseEvents(addressBuilder, chain.Chain, eventsI)
	require.NoError(t, err)
	require.Len(t, sources, 0)
	require.Len(t, dests, 0)

	stakes, _, err := api.ParseStakingEvents(addressBuilder, chain.Chain, eventsI)
	require.Len(t, stakes, 1)
	require.Equal(t, stakes[0].Validator, "5HjBSeeoz52CLfvDWDkzupqrYLHz1oToDPHjdmJjc4TF68LQ")
	require.Equal(t, stakes[0].Balance.String(), "3250000")

	_, fee, ok, err := api.ParseFee(addressBuilder, eventsI)
	require.True(t, ok, "has fee")
	require.Equal(t, fee.String(), "1234")
}
