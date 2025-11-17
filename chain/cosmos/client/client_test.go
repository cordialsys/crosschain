package client_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/cosmos/client"
	"github.com/cordialsys/crosschain/chain/cosmos/tx"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input/gas"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	testtypes "github.com/cordialsys/crosschain/testutil"
	"github.com/cosmos/cosmos-sdk/types"
	cosmostx "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

type TxInput = tx_input.TxInput

func TestNewClient(t *testing.T) {
	client, err := client.NewClient(xc.NewChainConfig(""))
	require.NotNil(t, client)
	require.NoError(t, err)
}

func makeSimulateResponse(gasUsed uint64, gasRequested uint64) string {
	response := cosmostx.SimulateResponse{
		GasInfo: &types.GasInfo{
			GasUsed:   gasUsed,
			GasWanted: gasRequested,
		},
		Result: &types.Result{},
	}
	responseBz, err := response.Marshal()
	if err != nil {
		panic(err)
	}

	jsonRpcResult := fmt.Sprintf(`{"response": {
		"code":0,
		"log":"",
		"info":"",
		"index":"0",
		"key":null,
		"value":"%s",
		"proofOps":null,
		"height":"13534747",
		"codespace":""}
	}
	`, base64.StdEncoding.EncodeToString(responseBz))
	return jsonRpcResult
}
func TestFetchTxInput(t *testing.T) {

	vectors := []struct {
		asset     *xc.ChainConfig
		from      string
		pubKeyStr string
		to        string
		feePayer  string
		resp      interface{}
		txInput   *tx_input.TxInput
		err       string
	}{
		{
			asset:     xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			from:      "terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
			pubKeyStr: "Avz3JMl9/6wgIe+hgYwv7zvLt1PKIpE6jbXnnsSj3uDR",
			to:        "terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn",
			resp: []string{
				`{"jsonrpc":"2.0","id":0,"result":{"response":{"code":0,"log":"","info":"","index":"0","key":null,"value":"CqABCiAvY29zbW9zLmF1dGgudjFiZXRhMS5CYXNlQWNjb3VudBJ8Cix0ZXJyYTFkcDNxMzA1aGd0dHQ4bjM0cnQ4cmc5eHBhbmM0Mno0eWU3dXBmZxJGCh8vY29zbW9zLmNyeXB0by5zZWNwMjU2azEuUHViS2V5EiMKIQL89yTJff+sICHvoYGML+87y7dTyiKROo21557Eo97g0RjZhgEgAw==","proofOps":null,"height":"2803726","codespace":""}}}`,
				`{"jsonrpc": "2.0","id": 1,"result": {"node_info": {"protocol_version": {},"network": "chainId"},"sync_info": {"latest_block_height": "123"},"validator_info": {}}}`,
				`{"jsonrpc":"2.0","id":2,"result":{"code":13,"data":"","log":"insufficient fees; got: 0uluna required: 15000uluna: insufficient fee","codespace":"sdk","hash":"C96E183E5FE6288EFA254C8003F5DD37D3EA51889E09F45CAA0749EF6FE25420"}}`,
				// get x/bank balance
				`{"jsonrpc":"2.0","id":3,"result":{"response":{"code":0,"log":"","info":"","index":"0","key":null,"value":"ChAKBXVsdW5hEgc0OTc5MDYz","proofOps":null,"height":"12817698","codespace":""}}}`,
				// get tx simulate
				fmt.Sprintf(`{"jsonrpc":"2.0","id":4,"result":%s}`, makeSimulateResponse(50_000, 100_000)),
			},
			txInput: &tx_input.TxInput{
				TxInputEnvelope: xc.TxInputEnvelope{Type: "cosmos"},
				AccountNumber:   17241,
				Sequence:        3,
				GasLimit:        (50_000 * client.DefaultGasLimitMultiplier),
				GasPrice:        0.015,
				TimeoutHeight:   client.TimeoutInBlocks + 123,
				AssetType:       tx_input.BANK,
				ChainId:         "chainId",
			},
			err: "",
		},
		{
			asset:     xc.NewChainConfig(xc.XPLA).WithChainCoin("axpla").WithChainPrefix("xpla").WithDriver(xc.DriverCosmosEvmos),
			from:      "xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg",
			pubKeyStr: "AreNsVEsIEpsORnscZlxzo7Xha4JRK0a7v6rJwPR5U0C",
			to:        "xpla1a8f3wnn7qwvwdzxkc9w849kfzhrr6gdvy4c8wv",
			resp: []string{
				`{"jsonrpc":"2.0","id":0,"result":{"response":{"code":0,"log":"","info":"","index":"0","key":null,"value":"CqgBCiAvY29zbW9zLmF1dGgudjFiZXRhMS5CYXNlQWNjb3VudBKDAQoreHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZxJPCigvZXRoZXJtaW50LmNyeXB0by52MS5ldGhzZWNwMjU2azEuUHViS2V5EiMKIQK3jbFRLCBKbDkZ7HGZcc6O14WuCUStGu7+qycD0eVNAhiiCyAE","proofOps":null,"height":"1359950","codespace":""}}}`,
				`{"jsonrpc": "2.0","id": 1,"result": {"node_info": {"protocol_version": {},"network": "chainId"},"sync_info": {},"validator_info": {}}}`,
				`{"jsonrpc":"2.0","id":2,"result":{"code":13,"data":"","log":"insufficient fees; got: 0axpla required: 850000000000axpla: insufficient fee","codespace":"sdk","hash":"C96E183E5FE6288EFA254C8003F5DD37D3EA51889E09F45CAA0749EF6FE25420"}}`,
				// get x/bank balance
				`{"jsonrpc":"2.0","id":3,"result":{"response":{"code":0,"log":"","info":"","index":"0","key":null,"value":"ChAKBXVsdW5hEgc0OTc5MDYz","proofOps":null,"height":"12817698","codespace":""}}}`,
				// get tx simulate
				fmt.Sprintf(`{"jsonrpc":"2.0","id":4,"result":%s}`, makeSimulateResponse(50_000, 100_000)),
			},
			txInput: &tx_input.TxInput{
				TxInputEnvelope: xc.TxInputEnvelope{Type: "cosmos"},
				AccountNumber:   1442,
				Sequence:        4,
				GasLimit:        (50_000 * client.DefaultGasLimitMultiplier),
				TimeoutHeight:   client.TimeoutInBlocks,
				GasPrice:        850000,
				AssetType:       tx_input.BANK,
				ChainId:         "chainId",
			},
			err: "",
		},
		// CW20 token type
		{
			asset:     xc.NewChainConfig(xc.XPLA).WithChainCoin("axpla").WithChainPrefix("xpla").WithDriver(xc.DriverCosmosEvmos),
			from:      "xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg",
			pubKeyStr: "AreNsVEsIEpsORnscZlxzo7Xha4JRK0a7v6rJwPR5U0C",
			to:        "xpla1a8f3wnn7qwvwdzxkc9w849kfzhrr6gdvy4c8wv",
			resp: []string{
				// get-account
				`{"jsonrpc":"2.0","id":0,"result":{"response":{"code":0,"log":"","info":"","index":"0","key":null,"value":"CqgBCiAvY29zbW9zLmF1dGgudjFiZXRhMS5CYXNlQWNjb3VudBKDAQoreHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZxJPCigvZXRoZXJtaW50LmNyeXB0by52MS5ldGhzZWNwMjU2azEuUHViS2V5EiMKIQK3jbFRLCBKbDkZ7HGZcc6O14WuCUStGu7+qycD0eVNAhiiCyAE","proofOps":null,"height":"1359950","codespace":""}}}`,
				// chain status
				`{"jsonrpc": "2.0","id": 1,"result": {"node_info": {"protocol_version": {},"network": "chainId"},"sync_info": {},"validator_info": {}}}`,
				// fee estimation
				`{"jsonrpc":"2.0","id":2,"result":{"code":13,"data":"","log":"insufficient fees; got: 0axpla required: 850000000000axpla: insufficient fee","codespace":"sdk","hash":"C96E183E5FE6288EFA254C8003F5DD37D3EA51889E09F45CAA0749EF6FE25420"}}`,
				// get x/bank balance (fail)
				`{"jsonrpc":"2.0","id":3,"result":{"response":{"code":1,"log":"","info":"bad denom","index":"0","key":null,"value":"","proofOps":null,"height":"12817698","codespace":""}}}`,
				// get x/wasm cw20 balance
				`{"jsonrpc":"2.0","id":4,"result":{"response":{"code":0,"log":"","info":"","index":"0","key":null,"value":"ChZ7ImJhbGFuY2UiOiI0Mzk4NDEyNyJ9","proofOps":null,"height":"12817698","codespace":""}}}`,
				// get tx simulate
				fmt.Sprintf(`{"jsonrpc":"2.0","id":5,"result":%s}`, makeSimulateResponse(50_000, 100_000)),
			},
			txInput: &tx_input.TxInput{
				TxInputEnvelope: xc.TxInputEnvelope{Type: "cosmos"},
				AccountNumber:   1442,
				Sequence:        4,
				GasLimit:        (50_000 * client.DefaultGasLimitMultiplier),
				TimeoutHeight:   client.TimeoutInBlocks,
				GasPrice:        850000,
				AssetType:       tx_input.CW20,
				ChainId:         "chainId",
			},
			err: "",
		},
		{
			// fee-payer
			asset:     xc.NewChainConfig(xc.XPLA).WithChainCoin("axpla").WithChainPrefix("xpla").WithDriver(xc.DriverCosmosEvmos),
			from:      "xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg",
			pubKeyStr: "AreNsVEsIEpsORnscZlxzo7Xha4JRK0a7v6rJwPR5U0C",
			to:        "xpla1a8f3wnn7qwvwdzxkc9w849kfzhrr6gdvy4c8wv",
			feePayer:  "xpla1ku8pgxxga27x6v62gvu4h9dpnk7y8kcp92grl0",
			resp: []string{
				// get-account
				`{"jsonrpc":"2.0","id":0,"result":{"response":{"code":0,"log":"","info":"","index":"0","key":null,"value":"CqgBCiAvY29zbW9zLmF1dGgudjFiZXRhMS5CYXNlQWNjb3VudBKDAQoreHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZxJPCigvZXRoZXJtaW50LmNyeXB0by52MS5ldGhzZWNwMjU2azEuUHViS2V5EiMKIQK3jbFRLCBKbDkZ7HGZcc6O14WuCUStGu7+qycD0eVNAhiiCyAE","proofOps":null,"height":"1359950","codespace":""}}}`,
				// get-account (fee-payer)
				`{"jsonrpc":"2.0","id":1,"result":{"response":{"code":0,"log":"","info":"","index":"0","key":null,"value":"CqgBCiAvY29zbW9zLmF1dGgudjFiZXRhMS5CYXNlQWNjb3VudBKDAQoreHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZxJPCigvZXRoZXJtaW50LmNyeXB0by52MS5ldGhzZWNwMjU2azEuUHViS2V5EiMKIQK3jbFRLCBKbDkZ7HGZcc6O14WuCUStGu7+qycD0eVNAhiiCyAE","proofOps":null,"height":"1359950","codespace":""}}}`,
				// chain status
				`{"jsonrpc": "2.0","id": 2,"result": {"node_info": {"protocol_version": {},"network": "chainId"},"sync_info": {},"validator_info": {}}}`,
				// fee estimation
				`{"jsonrpc":"2.0","id":3,"result":{"code":13,"data":"","log":"insufficient fees; got: 0axpla required: 850000000000axpla: insufficient fee","codespace":"sdk","hash":"C96E183E5FE6288EFA254C8003F5DD37D3EA51889E09F45CAA0749EF6FE25420"}}`,
				// get x/bank balance
				`{"jsonrpc":"2.0","id":4,"result":{"response":{"code":0,"log":"","info":"","index":"0","key":null,"value":"ChAKBXVsdW5hEgc0OTc5MDYz","proofOps":null,"height":"12817698","codespace":""}}}`,
				// get tx simulate
				fmt.Sprintf(`{"jsonrpc":"2.0","id":5,"result":%s}`, makeSimulateResponse(50_000, 100_000)),
			},
			txInput: &tx_input.TxInput{
				TxInputEnvelope:       xc.TxInputEnvelope{Type: "cosmos"},
				AccountNumber:         1442,
				Sequence:              4,
				FeePayerAccountNumber: 1442,
				FeePayerSequence:      4,
				GasLimit:              (50_000 * client.DefaultGasLimitMultiplier),
				TimeoutHeight:         client.TimeoutInBlocks,
				GasPrice:              850000,
				AssetType:             tx_input.BANK,
				ChainId:               "chainId",
			},
			err: "",
		},
		{
			asset:     xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			from:      "terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
			pubKeyStr: "Avz3JMl9/6wgIe+hgYwv7zvLt1PKIpE6jbXnnsSj3uDR",
			to:        "terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn",
			resp: []string{
				`{"jsonrpc":"2.0","id":0,"result":{"response":{"code":0,"log":"","info":"","index":"0","key":null,"value":"CqABCiAvY29zbW9zLmF1dGgudjFiZXRhMS5CYXNlQWNjb3VudBJ8Cix0ZXJyYTFkcDNxMzA1aGd0dHQ4bjM0cnQ4cmc5eHBhbmM0Mno0eWU3dXBmZxJGCh8vY29zbW9zLmNyeXB0by5zZWNwMjU2azEuUHViS2V5EiMKIQL89yTJff+sICHvoYGML+87y7dTyiKROo21557Eo97g0RjZhgEgAw==","proofOps":null,"height":"2803726","codespace":""}}}`,
				`{"jsonrpc": "2.0","id": 1,"result": {"node_info": {"protocol_version": {},"network": "chainId"},"sync_info": {"latest_block_height": "123"},"validator_info": {}}}`,
				`{"jsonrpc":"2.0","id":2,"result":{"code":13,"data":"","log":"insufficient fees; got: 0uluna required: 15000uluna: insufficient fee","codespace":"sdk","hash":"C96E183E5FE6288EFA254C8003F5DD37D3EA51889E09F45CAA0749EF6FE25420"}}`,
				// get x/bank balance
				`{"jsonrpc":"2.0","id":3,"result":{"response":{"code":0,"log":"","info":"","index":"0","key":null,"value":"ChAKBXVsdW5hEgc0OTc5MDYz","proofOps":null,"height":"12817698","codespace":""}}}`,
				// get tx simulate
				`{"jsonrpc":"2.0","id":4,"error":{"message": "account sequence mismatch: ...", "code": 123}}`,
				`{"jsonrpc":"2.0","id":4,"error":{"message": "account sequence mismatch: ...", "code": 123}}`,
			},
			txInput: &tx_input.TxInput{
				TxInputEnvelope: xc.TxInputEnvelope{Type: "cosmos"},
				AccountNumber:   17241,
				Sequence:        3,
				GasLimit:        (50_000 * client.DefaultGasLimitMultiplier),
				GasPrice:        0.015,
				TimeoutHeight:   client.TimeoutInBlocks + 123,
				AssetType:       tx_input.BANK,
				ChainId:         "chainId",
			},
			// should identify the error as retryable
			err: fmt.Sprintf("%s:", errors.FailedPrecondition),
		},
		// error getting account from RPC
		{
			asset:     xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			from:      "terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
			pubKeyStr: "",
			to:        "terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn",
			resp:      ``,
			txInput:   &tx_input.TxInput{TxInputEnvelope: xc.TxInputEnvelope{Type: "cosmos"}},
			err:       "failed to get account data",
		},
		{
			asset:     xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			from:      "terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
			pubKeyStr: "",
			to:        "terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn",
			resp:      `null`,
			txInput:   &tx_input.TxInput{TxInputEnvelope: xc.TxInputEnvelope{Type: "cosmos"}},
			err:       "failed to get account data",
		},
		{
			asset:     xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			from:      "terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
			pubKeyStr: "",
			to:        "terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn",
			resp:      `{}`,
			txInput:   &tx_input.TxInput{TxInputEnvelope: xc.TxInputEnvelope{Type: "cosmos"}},
			err:       "failed to get account data",
		},
		{
			asset:     xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			from:      "terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
			pubKeyStr: "",
			to:        "terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn",
			resp:      `{"message": "custom RPC error", "code": 123}`,
			txInput:   &tx_input.TxInput{TxInputEnvelope: xc.TxInputEnvelope{Type: "cosmos"}},
			err:       "failed to get account data",
		},
		// error getting gas
		{
			asset:     xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			from:      "terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
			pubKeyStr: "",
			to:        "terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn",
			resp: []string{
				`{"jsonrpc":"2.0","id":0,"result":{"response":{"code":0,"log":"","info":"","index":"0","key":null,"value":"CqABCiAvY29zbW9zLmF1dGgudjFiZXRhMS5CYXNlQWNjb3VudBJ8Cix0ZXJyYTFkcDNxMzA1aGd0dHQ4bjM0cnQ4cmc5eHBhbmM0Mno0eWU3dXBmZxJGCh8vY29zbW9zLmNyeXB0by5zZWNwMjU2azEuUHViS2V5EiMKIQL89yTJff+sICHvoYGML+87y7dTyiKROo21557Eo97g0RjZhgEgAw==","proofOps":null,"height":"2803726","codespace":""}}}`,
				`{"jsonrpc": "2.0","id": 1,"result": {"node_info": {"protocol_version": {},"network": "chainId"},"sync_info": {},"validator_info": {}}}`,
				`{"message": "custom HTTP error", "code": 123}`,
			},
			txInput: &TxInput{
				TxInputEnvelope: xc.TxInputEnvelope{Type: "cosmos"},
				AccountNumber:   17241,
				Sequence:        3,
				GasLimit:        gas.NativeTransferGasLimit,
				TimeoutHeight:   client.TimeoutInBlocks,
				ChainId:         "chainId",
			},
			err: "failed to estimate gas",
		},
		{
			asset:     xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			from:      "terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
			pubKeyStr: "",
			to:        "terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn",
			resp: []string{
				`{"jsonrpc":"2.0","id":0,"result":{"response":{"code":0,"log":"","info":"","index":"0","key":null,"value":"CqABCiAvY29zbW9zLmF1dGgudjFiZXRhMS5CYXNlQWNjb3VudBJ8Cix0ZXJyYTFkcDNxMzA1aGd0dHQ4bjM0cnQ4cmc5eHBhbmM0Mno0eWU3dXBmZxJGCh8vY29zbW9zLmNyeXB0by5zZWNwMjU2azEuUHViS2V5EiMKIQL89yTJff+sICHvoYGML+87y7dTyiKROo21557Eo97g0RjZhgEgAw==","proofOps":null,"height":"2803726","codespace":""}}}`,
				`{"jsonrpc": "2.0","id": 1,"result": {"node_info": {"protocol_version": {},"network": "chainId"},"sync_info": {},"validator_info": {}}}`,
				"null",
			},
			txInput: &tx_input.TxInput{
				TxInputEnvelope: xc.TxInputEnvelope{Type: "cosmos"},
				AccountNumber:   17241,
				GasLimit:        gas.NativeTransferGasLimit,
				TimeoutHeight:   client.TimeoutInBlocks,
				Sequence:        3,
				ChainId:         "chainId",
			},
			err: "failed to estimate gas",
		},
	}

	for i, v := range vectors {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			server, close := testtypes.MockJSONRPC(t, v.resp)
			defer close()

			v.asset.URL = server.URL
			v.asset.Limiter = rate.NewLimiter(rate.Inf, 1)
			client, err := client.NewClient(v.asset)
			require.NoError(t, err)
			from := xc.Address(v.from)
			to := xc.Address(v.to)
			args, err := xcbuilder.NewTransferArgs(v.asset.Base(), from, to, xc.NewAmountBlockchainFromUint64(1))
			require.NoError(t, err)

			if v.feePayer != "" {
				args.SetFeePayer(xc.Address(v.feePayer))
			}

			input, err := client.FetchTransferInput(context.Background(), args)

			if v.err != "" {
				require.ErrorContains(t, err, v.err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)

				require.Equal(t, v.txInput, input)
			}
		})
	}
}

func TestSubmitTxErr(t *testing.T) {

	client, _ := client.NewClient(xc.NewChainConfig(""))
	tx := &tx.Tx{ChainCfg: xc.NewChainConfig("").Base()}
	req, err := xctypes.SubmitTxReqFromTx(tx)
	require.NoError(t, err)
	err = client.SubmitTx(context.Background(), req)
	require.ErrorContains(t, err, "no Host in request URL")
}

func TestFetchTxInfo(t *testing.T) {

	vectors := []struct {
		asset xc.ITask
		tx    string
		resp  interface{}
		val   txinfo.LegacyTxInfo
		err   string
	}{
		{
			// receive LUNA from faucet
			xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			"E9C24C2E23CDCA56C8CE87A583149F8F88E75923F0CD958C003A84F631948978",
			[]string{
				// tx
				`{"jsonrpc":"2.0","id":0,"result":{"hash":"E9C24C2E23CDCA56C8CE87A583149F8F88E75923F0CD958C003A84F631948978","height":"2754866","index":1,"tx_result":{"code":0,"data":"Ch4KHC9jb3Ntb3MuYmFuay52MWJldGExLk1zZ1NlbmQ=","log":"[{\"events\":[{\"type\":\"coin_received\",\"attributes\":[{\"key\":\"receiver\",\"value\":\"terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg\"},{\"key\":\"amount\",\"value\":\"5000000uluna\"}]},{\"type\":\"coin_spent\",\"attributes\":[{\"key\":\"spender\",\"value\":\"terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn\"},{\"key\":\"amount\",\"value\":\"5000000uluna\"}]},{\"type\":\"message\",\"attributes\":[{\"key\":\"action\",\"value\":\"/cosmos.bank.v1beta1.MsgSend\"},{\"key\":\"sender\",\"value\":\"terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn\"},{\"key\":\"module\",\"value\":\"bank\"}]},{\"type\":\"transfer\",\"attributes\":[{\"key\":\"recipient\",\"value\":\"terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg\"},{\"key\":\"sender\",\"value\":\"terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn\"},{\"key\":\"amount\",\"value\":\"5000000uluna\"}]}]}]","info":"","gas_wanted":"150000","gas_used":"80283","events":[{"type":"coin_spent","attributes":[{"key":"c3BlbmRlcg==","value":"dGVycmExaDhsamRtYWU3bHgwNWtqajc5Yzlla3Njd3N5amQzeXI4d3l2ZG4=","index":true},{"key":"YW1vdW50","value":"MTAwMDAwMHVsdW5h","index":true}]},{"type":"coin_received","attributes":[{"key":"cmVjZWl2ZXI=","value":"dGVycmExN3hwZnZha20yYW1nOTYyeWxzNmY4NHoza2VsbDhjNWxrYWVxZmE=","index":true},{"key":"YW1vdW50","value":"MTAwMDAwMHVsdW5h","index":true}]},{"type":"transfer","attributes":[{"key":"cmVjaXBpZW50","value":"dGVycmExN3hwZnZha20yYW1nOTYyeWxzNmY4NHoza2VsbDhjNWxrYWVxZmE=","index":true},{"key":"c2VuZGVy","value":"dGVycmExaDhsamRtYWU3bHgwNWtqajc5Yzlla3Njd3N5amQzeXI4d3l2ZG4=","index":true},{"key":"YW1vdW50","value":"MTAwMDAwMHVsdW5h","index":true}]},{"type":"message","attributes":[{"key":"c2VuZGVy","value":"dGVycmExaDhsamRtYWU3bHgwNWtqajc5Yzlla3Njd3N5amQzeXI4d3l2ZG4=","index":true}]},{"type":"tx","attributes":[{"key":"ZmVl","value":"MTAwMDAwMHVsdW5h","index":true}]},{"type":"tx","attributes":[{"key":"YWNjX3NlcQ==","value":"dGVycmExaDhsamRtYWU3bHgwNWtqajc5Yzlla3Njd3N5amQzeXI4d3l2ZG4vMTc3ODE=","index":true}]},{"type":"tx","attributes":[{"key":"c2lnbmF0dXJl","value":"NjlXMDNraHFhbElhRS9mbmg3YjdtM1pQaEpFTDhDWk9FQTdQK0dJT2M3ZDE4eVh3N1phZWVnbk5USU8rRW0wWFZsUi95bTFpbDkvUTZMZGRoWDAyREE9PQ==","index":true}]},{"type":"message","attributes":[{"key":"YWN0aW9u","value":"L2Nvc21vcy5iYW5rLnYxYmV0YTEuTXNnU2VuZA==","index":true}]},{"type":"coin_spent","attributes":[{"key":"c3BlbmRlcg==","value":"dGVycmExaDhsamRtYWU3bHgwNWtqajc5Yzlla3Njd3N5amQzeXI4d3l2ZG4=","index":true},{"key":"YW1vdW50","value":"NTAwMDAwMHVsdW5h","index":true}]},{"type":"coin_received","attributes":[{"key":"cmVjZWl2ZXI=","value":"dGVycmExZHAzcTMwNWhndHR0OG4zNHJ0OHJnOXhwYW5jNDJ6NHllN3VwZmc=","index":true},{"key":"YW1vdW50","value":"NTAwMDAwMHVsdW5h","index":true}]},{"type":"transfer","attributes":[{"key":"cmVjaXBpZW50","value":"dGVycmExZHAzcTMwNWhndHR0OG4zNHJ0OHJnOXhwYW5jNDJ6NHllN3VwZmc=","index":true},{"key":"c2VuZGVy","value":"dGVycmExaDhsamRtYWU3bHgwNWtqajc5Yzlla3Njd3N5amQzeXI4d3l2ZG4=","index":true},{"key":"YW1vdW50","value":"NTAwMDAwMHVsdW5h","index":true}]},{"type":"message","attributes":[{"key":"c2VuZGVy","value":"dGVycmExaDhsamRtYWU3bHgwNWtqajc5Yzlla3Njd3N5amQzeXI4d3l2ZG4=","index":true}]},{"type":"message","attributes":[{"key":"bW9kdWxl","value":"YmFuaw==","index":true}]}],"codespace":""},"tx":"CpkBCo4BChwvY29zbW9zLmJhbmsudjFiZXRhMS5Nc2dTZW5kEm4KLHRlcnJhMWg4bGpkbWFlN2x4MDVramo3OWM5ZWtzY3dzeWpkM3lyOHd5dmRuEix0ZXJyYTFkcDNxMzA1aGd0dHQ4bjM0cnQ4cmc5eHBhbmM0Mno0eWU3dXBmZxoQCgV1bHVuYRIHNTAwMDAwMBIGZmF1Y2V0EmwKUgpGCh8vY29zbW9zLmNyeXB0by5zZWNwMjU2azEuUHViS2V5EiMKIQKv7tshoUn8AjeXjcz+FdLCDlGOt3aB6uKlr5qXPoPYkxIECgIIARj1igESFgoQCgV1bHVuYRIHMTAwMDAwMBDwkwkaQOvVtN5IampSGhP354e2+5t2T4SRC/AmThAOz/hiDnO3dfMl8O2WnnoJzUyDvhJtF1ZUf8ptYpff0Oi3XYV9Ngw="}}`,
				// block
				`{"jsonrpc":"2.0","id":0,"result":{"block_id":{"hash":"55DF5840E4D24A53DF08E7D7D2B99DDAC9B60F2A683AF12542F1446E9966599A","parts":{"total":1,"hash":"C246132CBEEE1A8AD9B05917D945F7EF82F23987BFC80F2D850DEBA63F8AE873"}},"block":{"header":{"version":{"block":"11"},"chain_id":"pisco-1","height":"2800210","time":"2022-11-19T20:56:02.700490668Z","last_block_id":{"hash":"31B7B3982282E6572A65E41CE45B9829E5B7646DB2C46B277649275A51281E5C","parts":{"total":1,"hash":"048EAF0E0EECD416D3F8F78D3A4C953D0CC3C2F8707F3682BB9F8BBB1D6BA300"}},"last_commit_hash":"0D3F0281AEC1E8E96F581F0B926B7106FB5D6F5A025D3C1F633639C19DFEBD38","data_hash":"E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855","validators_hash":"00171720E1B095C40D42ABEAF3F036003CA888A4D67DEA6B1EF44A06A95B4D08","next_validators_hash":"00171720E1B095C40D42ABEAF3F036003CA888A4D67DEA6B1EF44A06A95B4D08","consensus_hash":"E660EF14A95143DB0F3EAF2F31F177DE334DE5AB650579FD093A10CBAE86D5A6","app_hash":"25FC61CC0AE05F3B96AF290F8AAD21086D7F4C6947C0E6A9395DF4DDA070C6E1","last_results_hash":"E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855","evidence_hash":"E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855","proposer_address":"C24A7D204E0A07736EAF8A7E76820CD868565B0E"},"data":{"txs":[]},"evidence":{"evidence":[]},"last_commit":{"height":"2800209","round":0,"block_id":{"hash":"31B7B3982282E6572A65E41CE45B9829E5B7646DB2C46B277649275A51281E5C","parts":{"total":1,"hash":"048EAF0E0EECD416D3F8F78D3A4C953D0CC3C2F8707F3682BB9F8BBB1D6BA300"}},"signatures":[{"block_id_flag":2,"validator_address":"B384CE5A8F860736EFE9C1C467101D8413B90B81","timestamp":"2022-11-19T20:56:02.700490668Z","signature":"sRC64oT2D6iCS8sRgEZHTiARjDYQUmN2SJ8cBmBl52yz/sRfTxbnaiPq+U+HMC7hpHvcSxuJPl+EY5MrlcisBQ=="},{"block_id_flag":2,"validator_address":"C24A7D204E0A07736EAF8A7E76820CD868565B0E","timestamp":"2022-11-19T20:56:02.697154012Z","signature":"VLAf/hDgy5k9Cag4YXXPJ60mH2pgKQz9IwaxNjcKy85h5TbUAevAWU175RBcZs+LBrHTcKiWRe3pUpbNpsXICA=="},{"block_id_flag":2,"validator_address":"EA45D3A9C56AE8217795E0A819380848426D9825","timestamp":"2022-11-19T20:56:02.717871496Z","signature":"jZmfVTSy0dRq18Jr2TBn98at+w6661vPafSTBNBeeKauKcQkA7Dr7mkWUm1eybBxLxzapyeGCEx6cpj30lEMAg=="},{"block_id_flag":2,"validator_address":"CA861E2E59AE3D7D998ADB7716C91419E032F7FC","timestamp":"2022-11-19T20:56:02.84915447Z","signature":"myXk7hns3LZ3gtKPcWN5VyqvL5NpUmDtzAxANXbNxc6TjHdK1kh7sp4xGchwEfP++/gSieRMTmneO0TapnXYDg=="},{"block_id_flag":2,"validator_address":"A39FD495DBBE30110E139C7B6EF6CB094228EA20","timestamp":"2022-11-19T20:56:02.70712284Z","signature":"fOe9kqTEpuECxb+2e5p5bm6BHZQECL+lX1VaBkJbN0WjNIWDjIOznfnvVhtMGg/WLWI2J5veQ8XsaWcHbse+BA=="},{"block_id_flag":2,"validator_address":"C3DE9695E9A7B20CB96F4C3FB418E0B819941D2B","timestamp":"2022-11-19T20:56:02.701424437Z","signature":"YlZxYlH+LUFuQMBQ/JgE5CNWN9AhQzlz8VjCxoBbU5TVkajQdpP50j97Ho5z4K130lVYTxtcsQPJYWy2RBygBw=="},{"block_id_flag":2,"validator_address":"193930827BBD3CC18727D77F3F850B6B6087294A","timestamp":"2022-11-19T20:56:02.788661493Z","signature":"QAH/mFV6Nm9SaNMPkgFMCnavm3db+Oyqy9WovdBKkAzwu6vKDjyaQtiMLQ/WUdxSVH3Z//2oH1NKlAxiQ2FKDw=="},{"block_id_flag":2,"validator_address":"E36B8058A160B7592F487556B04D0B2FCF55BB21","timestamp":"2022-11-19T20:56:02.721541112Z","signature":"m6GTVzOceWM97zuJFWu20YArzgVn/MRpVcavreLiEUrIkqqRI36uj4ZZc96EbNCMRy7QB8B3AT8e/zKj4zM1AA=="},{"block_id_flag":2,"validator_address":"6ACAE281C0E936871FAA670F2209561E17A11071","timestamp":"2022-11-19T20:56:02.735968292Z","signature":"qQHSLcrobsZbf2ZOUOdu74TOOx4kd/Hz96jl/V6QQRM29P0D67NUNgkhsOCiLFnmdy0+SraSzsdJDSx+CXb0BA=="},{"block_id_flag":2,"validator_address":"012CD164F20D118EDFEA622407EAFD9DC1A27873","timestamp":"2022-11-19T20:56:02.785908575Z","signature":"eauzlWFWwCUh+1yxwzlz3LahAhArVgfoOsFU4zDvXV3G6dnmiBPseuy5HzlS7c6AKv1XEBiFASbFfRJoraIfDA=="},{"block_id_flag":2,"validator_address":"19AA44E5553DED864BE3371D135416B085C795E6","timestamp":"2022-11-19T20:56:02.716000863Z","signature":"H8dwCyQoLOGEvnNX2Q9MTF7WMEhnheVf4wfS5wqv0Trmgc2fLqml/dimy83vWm7tj0HHNFm22kawiljVyGU5Cw=="},{"block_id_flag":2,"validator_address":"744C7305044AC0B439088D991F44C73F827F4D0E","timestamp":"2022-11-19T20:56:02.696850058Z","signature":"SjtkmD626Cg2JkIcFejU6n1qqJIGIyqxGinRqmykZ+uyIbRNytZ79azZpQkarEU7GryL6q3mooXuChi62GXNBA=="},{"block_id_flag":2,"validator_address":"0075E9DB4870193B9683711614BBABC497C31AA3","timestamp":"2022-11-19T20:56:02.739841071Z","signature":"xOiYBtwSi1s34vy476Dh/j6GOHCv/kq48WZOueV4VEq/x5C+TIm6+fN8oOhjHFavPczG+TcMod9dz0ZQbIuzAQ=="},{"block_id_flag":2,"validator_address":"62D61731A2D9093CFFDCC4D22765F26F0E9CBEF3","timestamp":"2022-11-19T20:56:02.84600571Z","signature":"xgM7v69lEP0nQ8Z7hWhqcs4v5jDnBXqUlcnxeRcYSZNnUPSqDnbfF+VdyLq0uaC2bBzXCmyzGl1Hq99F0C4BBg=="},{"block_id_flag":2,"validator_address":"22F1EAA184BD177E66E62B53DF65EE088DFB4ECB","timestamp":"2022-11-19T20:56:02.74530502Z","signature":"OPlLXuWNKqzDef+fvUkp+9YjV9pCmdJiyweHAGqZ/JrR8e+/LgPoW3WeCYrHsvBeSQ44bQgM9PwRlacnwL0BCg=="},{"block_id_flag":2,"validator_address":"51E922FC1DD642631A81ADA37D829F6D04656F4A","timestamp":"2022-11-19T20:56:02.789660749Z","signature":"TN736W/ATvowmt5rgp9zpobIJYvlz7rZQdWN9hwe91ywbjISVSeP8k+bqWafspicUBeed5O5yqoQ0MiShj6KBA=="},{"block_id_flag":2,"validator_address":"122686EF1BBE42A167AFD568A92070B6C1F1FE77","timestamp":"2022-11-19T20:56:02.738700857Z","signature":"U/bD3+gljwFc7X74uJiS4aRzXD9+4ctYrHxO/OuklOZhRpaGPcZ7X9cBljv0seMxhggya8U9+m0k2SrIK1fkCg=="},{"block_id_flag":2,"validator_address":"4F045EA002C1110A5CAF2B23849C38D76E1AC0F3","timestamp":"2022-11-19T20:56:02.831904336Z","signature":"oHZmETE0cOWImSJNRa1JSW568BgPJCSXFfK60mkSxWxSTFA16o3dMabcdZxtSKkd8tfA2sEMWGefass5/sPGDQ=="},{"block_id_flag":2,"validator_address":"C4C2AB6DDDBFB6D86F5266531A441B39EB653FE7","timestamp":"2022-11-19T20:56:02.734419431Z","signature":"nkEmjhP1iihl/SYbIXwFPbHCEZMpMCq9X6OcWcVsq6wdmQlnaxrQo89EAlX8ekrc8lps38dv3NSO9hGJAK1dCw=="},{"block_id_flag":2,"validator_address":"CFE618B4BC8654819D5A4BB8A97CEAB70971D6B7","timestamp":"2022-11-19T20:56:02.745554138Z","signature":"/CK5jmjsHxq/sTiYuHPST2o6qeYz8nlHCmOnsofdWxDOFSteW7WXRy35imJ59sNYFBQe2DVDHnQeqSUHRg8nCQ=="},{"block_id_flag":2,"validator_address":"A2F1F94322A03D6EA83E7875D323BC8D629AEC8E","timestamp":"2022-11-19T20:56:02.710239793Z","signature":"sJlgNrMu0+gnAmJeD6MMz5+H37rZWyMhUwp/NJmRKQxAb/voPwev/BV4vkVO5DugfGEdBjwRuUNglEWahkTXBg=="},{"block_id_flag":2,"validator_address":"09F1BCA5A35FC45D0D0AD007310B4BD8994393AE","timestamp":"2022-11-19T20:56:02.7095822Z","signature":"WJC7f7mmE+Cc6Uw7C5cNDFlALkqIxYuqoaHeQ665Kk7hSsH9L+6d8luW/rBSpVN6FAMUqOIQ5mcTYLJX1wSZDQ=="},{"block_id_flag":2,"validator_address":"5D066416227488463F7090EC4E4909028D47086C","timestamp":"2022-11-19T20:56:02.710881115Z","signature":"9szkx6MQNoRIOLPvxWCS3t512EJtWVBQV4Uyw8IN4DewJnjY02ZH1gfEgN9yZriYe5NmU0zeR2ZBjXE1r/NCDg=="},{"block_id_flag":2,"validator_address":"1FEAD225509B0C3AC9F11F58CBD3FCA885265BEB","timestamp":"2022-11-19T20:56:02.796856612Z","signature":"t8qwYrRJeuKIKOEffuvu6mAfl8iOWsckMKDq/h8T+WWbGuI5gCrRTrA5gt4v3Cs4LzYfWlM2n3AqkRVqAjS8Cg=="},{"block_id_flag":2,"validator_address":"525BD01ACD7BC7D1FBE9B1D84EC691A08E60E427","timestamp":"2022-11-19T20:56:02.926033194Z","signature":"IfXy8OoG55y6rclJ7ufD4oTxt1Hy3995bO1r1n2KnSjf1WQmMfqlaUhh9cHN9eu18DvPn3sslD7ANmqyzUGXAA=="},{"block_id_flag":2,"validator_address":"6F30C69A5DCA7842311C0CF1B7100BFB081DF19A","timestamp":"2022-11-19T20:56:02.806425016Z","signature":"vAjUeXmLdOnodPpOO5Yh7yaL2hGg/AYNqIrzX3Mvmf8qvVwY4XYdewWujG+uPpn7ilnBiG4OHFgEIaBKKAD1Dw=="},{"block_id_flag":2,"validator_address":"7779E43DE5A3219F719E1C03D0511B679AC96CF8","timestamp":"2022-11-19T20:56:02.792300049Z","signature":"Td8c423mr3jN7/CKVo3fnzyxHXUIJJqsQcyukNpWLKDbYhSm5ju0JTooAe43Xw0lq3BTxi/n7CQPhBFFWy14BA=="},{"block_id_flag":2,"validator_address":"E8CE50BFF9543801CB4228B5B3AB5D8F617CE1D3","timestamp":"2022-11-19T20:56:02.793098718Z","signature":"id6xw8HbpEQRAoqDnkdXb7ZE5aKMx3lPdum1Tk3cNXnTyzCNh4rkTWb3t+bgMb/5fUzNS6w9S5NX1MyTPJvqBw=="},{"block_id_flag":2,"validator_address":"C7C3EF63B7DB35ED006504775421B5E1F3DE4473","timestamp":"2022-11-19T20:56:02.708518216Z","signature":"+sVaVLrf0ECyzKxm9aicEc+Ez3zm0Cn51m6939adlhI2levK/3FNAX1oL4PD8SRXhLL+acIZH9JtWDdRnmLGDg=="},{"block_id_flag":2,"validator_address":"426574C176F2CE22956C5FD53DC2E6A7773613A9","timestamp":"2022-11-19T20:56:02.737660078Z","signature":"fn64BWYgQYEGX+KRJBdfycoC6LWAaaeJkB0UTYTY1SCjeKI0WWp8e8qF4SPfT8T/ZxcCZliZt7brQpod+gu8Ag=="},{"block_id_flag":2,"validator_address":"D85FE0C08E590D06A5EA86407F1B10F361B85FED","timestamp":"2022-11-19T20:56:02.699160715Z","signature":"1HDprZp/VzNy1y8KK+PUhRvSisDRWhXOcK4rZFoa45xYbcBlVJXOt29q0+zJcT3tDjz3wH54BeP99mOZeIVPDQ=="},{"block_id_flag":2,"validator_address":"00FD6AA09300D18A0F0B91056CB645A8B3F488A0","timestamp":"2022-11-19T20:56:02.808657661Z","signature":"uq9vSymbQ0DS+kLgnXQ9Kx33eLPBtjMyEyA/kvMlOf0JIs1T1zVhVMBxGsxZt/HsYkhzXKmafCYnMgCS38vuAg=="},{"block_id_flag":2,"validator_address":"4B0B4CDF8201CEDCABF5FBD48375469614AEBF89","timestamp":"2022-11-19T20:56:02.702604433Z","signature":"Sed+vePNd+ePbG0hjqODzH75pFSxYq0iIqJi8im8Xd5RMtsGocoeLVcjCrZI/F44B5CjCf+IHzskFmZY803JAQ=="},{"block_id_flag":2,"validator_address":"3FEB6EA9117C4BCC545465EF930E658E80AA39D9","timestamp":"2022-11-19T20:56:02.84245292Z","signature":"BjUAjyNm/YIAPDJUj5U/K6Ga8Y+cxmtv5FMuESm6iLlbw6jqrL+w89e9pJCg3tahuM+RKT4BGGCx9nSTUW7uBg=="},{"block_id_flag":2,"validator_address":"155ABB8A90A9AF53B9EA617967A2FC1F432134C2","timestamp":"2022-11-19T20:56:02.715470406Z","signature":"cV3TeTykvyUYDY8jhH1FhuZxuODIhYZocK6auVxWMQg606OFRrpOetLk+ZnG03mDEgBWgLn20pqXo/1RZSdyAA=="},{"block_id_flag":2,"validator_address":"DFB4FF2582863145659FD7CDC78C2CB50F846A07","timestamp":"2022-11-19T20:56:02.696218736Z","signature":"EJHxDi2BnbZUvouoTR0Oi0ieIh8KyY8Z4D1pH3D4olaA7Q8tfQmhQvHarqbc9oU/l6dyIvHU414VsgWUe7VhDQ=="}]}}}}`,
				// abci_info
				`{"jsonrpc":"2.0","id":1,"result":{"response":{"data":"terra","version":"v2.2.0","last_block_height":"2803726","last_block_app_hash":"Ds7V/wiEMX5P06kXiX6Ye1G08MfLPJhdTXl95lBydZ0="}}}`,
			},
			txinfo.LegacyTxInfo{
				TxID:            "E9C24C2E23CDCA56C8CE87A583149F8F88E75923F0CD958C003A84F631948978",
				From:            "terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn",
				To:              "terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
				ToAlt:           "",
				ContractAddress: "LUNA",
				Amount:          xc.NewAmountBlockchainFromUint64(5000000),
				Fee:             xc.NewAmountBlockchainFromUint64(1000000),
				FeePayer:        "terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn",
				BlockIndex:      2754866,
				BlockTime:       1668891362,
				Confirmations:   48860,
				Status:          0,
				Sources: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address:         "terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn",
						ContractAddress: "LUNA",
						ContractId:      "uluna",
						Amount:          xc.NewAmountBlockchainFromUint64(5000000),
						Event:           txinfo.NewEventFromIndex(10, txinfo.MovementVariantNative),
					},
				},
				Destinations: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address:         "terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
						ContractAddress: "LUNA",
						ContractId:      "uluna",
						Amount:          xc.NewAmountBlockchainFromUint64(5000000),
						Memo:            "faucet",
						Event:           txinfo.NewEventFromIndex(10, txinfo.MovementVariantNative),
					},
				},
			},
			"",
		},
		{
			// send XPLA
			xc.NewChainConfig(xc.XPLA).WithChainCoin("axpla").WithChainPrefix("xpla"),
			"7a13cb946589d07834119e3d9f3bf27e38da9990894e24850323582a404de46b",
			[]string{
				// tx
				`{"jsonrpc":"2.0","id":0,"result":{"hash":"7A13CB946589D07834119E3D9F3BF27E38DA9990894E24850323582A404DE46B","height":"1359533","index":0,"tx_result":{"code":0,"data":"Ch4KHC9jb3Ntb3MuYmFuay52MWJldGExLk1zZ1NlbmQ=","log":"[{\"events\":[{\"type\":\"coin_received\",\"attributes\":[{\"key\":\"receiver\",\"value\":\"xpla1a8f3wnn7qwvwdzxkc9w849kfzhrr6gdvy4c8wv\"},{\"key\":\"amount\",\"value\":\"5000000000000000axpla\"}]},{\"type\":\"coin_spent\",\"attributes\":[{\"key\":\"spender\",\"value\":\"xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg\"},{\"key\":\"amount\",\"value\":\"5000000000000000axpla\"}]},{\"type\":\"message\",\"attributes\":[{\"key\":\"action\",\"value\":\"/cosmos.bank.v1beta1.MsgSend\"},{\"key\":\"sender\",\"value\":\"xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg\"},{\"key\":\"module\",\"value\":\"bank\"}]},{\"type\":\"transfer\",\"attributes\":[{\"key\":\"recipient\",\"value\":\"xpla1a8f3wnn7qwvwdzxkc9w849kfzhrr6gdvy4c8wv\"},{\"key\":\"sender\",\"value\":\"xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg\"},{\"key\":\"amount\",\"value\":\"5000000000000000axpla\"}]}]}]","info":"","gas_wanted":"110000","gas_used":"93146","events":[{"type":"coin_spent","attributes":[{"key":"c3BlbmRlcg==","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZw==","index":true},{"key":"YW1vdW50","value":"MTEyMjAwMDAwMDAwMDAwMDAwYXhwbGE=","index":true}]},{"type":"coin_received","attributes":[{"key":"cmVjZWl2ZXI=","value":"eHBsYTE3eHBmdmFrbTJhbWc5NjJ5bHM2Zjg0ejNrZWxsOGM1bHc3bXVxdw==","index":true},{"key":"YW1vdW50","value":"MTEyMjAwMDAwMDAwMDAwMDAwYXhwbGE=","index":true}]},{"type":"transfer","attributes":[{"key":"cmVjaXBpZW50","value":"eHBsYTE3eHBmdmFrbTJhbWc5NjJ5bHM2Zjg0ejNrZWxsOGM1bHc3bXVxdw==","index":true},{"key":"c2VuZGVy","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZw==","index":true},{"key":"YW1vdW50","value":"MTEyMjAwMDAwMDAwMDAwMDAwYXhwbGE=","index":true}]},{"type":"message","attributes":[{"key":"c2VuZGVy","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZw==","index":true}]},{"type":"tx","attributes":[{"key":"ZmVl","value":"MTEyMjAwMDAwMDAwMDAwMDAwYXhwbGE=","index":true},{"key":"ZmVlX3BheWVy","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZw==","index":true}]},{"type":"tx","attributes":[{"key":"YWNjX3NlcQ==","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZy8z","index":true}]},{"type":"tx","attributes":[{"key":"c2lnbmF0dXJl","value":"WGZSQnVQZHE3SWN1MTNieTBObjUxZlU2MUsyVkFSM2E2UGllMUJIZU1aUm1SR2p5aDRyNW9HK2VIQ3Y2R2EyWDUyd2tabmI2aUZVdXZNbjJVZ3Z2bnc9PQ==","index":true}]},{"type":"message","attributes":[{"key":"YWN0aW9u","value":"L2Nvc21vcy5iYW5rLnYxYmV0YTEuTXNnU2VuZA==","index":true}]},{"type":"coin_spent","attributes":[{"key":"c3BlbmRlcg==","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZw==","index":true},{"key":"YW1vdW50","value":"NTAwMDAwMDAwMDAwMDAwMGF4cGxh","index":true}]},{"type":"coin_received","attributes":[{"key":"cmVjZWl2ZXI=","value":"eHBsYTFhOGYzd25uN3F3dndkenhrYzl3ODQ5a2Z6aHJyNmdkdnk0Yzh3dg==","index":true},{"key":"YW1vdW50","value":"NTAwMDAwMDAwMDAwMDAwMGF4cGxh","index":true}]},{"type":"transfer","attributes":[{"key":"cmVjaXBpZW50","value":"eHBsYTFhOGYzd25uN3F3dndkenhrYzl3ODQ5a2Z6aHJyNmdkdnk0Yzh3dg==","index":true},{"key":"c2VuZGVy","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZw==","index":true},{"key":"YW1vdW50","value":"NTAwMDAwMDAwMDAwMDAwMGF4cGxh","index":true}]},{"type":"message","attributes":[{"key":"c2VuZGVy","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZw==","index":true}]},{"type":"message","attributes":[{"key":"bW9kdWxl","value":"YmFuaw==","index":true}]}],"codespace":""},"tx":"CpgBCpUBChwvY29zbW9zLmJhbmsudjFiZXRhMS5Nc2dTZW5kEnUKK3hwbGExaGR2ZjZ2djVhbWM3d3A4NGpzMGxzMjdhcGVrd3hwcjBnZTk2a2cSK3hwbGExYThmM3dubjdxd3Z3ZHp4a2M5dzg0OWtmemhycjZnZHZ5NGM4d3YaGQoFYXhwbGESEDUwMDAwMDAwMDAwMDAwMDASfgpZCk8KKC9ldGhlcm1pbnQuY3J5cHRvLnYxLmV0aHNlY3AyNTZrMS5QdWJLZXkSIwohAreNsVEsIEpsORnscZlxzo7Xha4JRK0a7v6rJwPR5U0CEgQKAggBGAMSIQobCgVheHBsYRISMTEyMjAwMDAwMDAwMDAwMDAwELDbBhpAXfRBuPdq7Icu13by0Nn51fU61K2VAR3a6Pie1BHeMZRmRGjyh4r5oG+eHCv6Ga2X52wkZnb6iFUuvMn2Ugvvnw=="}}`,
				// block
				`{"jsonrpc":"2.0","id":0,"result":{"block_id":{"hash":"9A1F181DF132ECF61AFC674063178DF42264C302EF78F33DBD5E83754ED30D4C","parts":{"total":1,"hash":"3470448F8E010EADB949CD81CD5DA5A04C55502FD856C60E38B8FA2CE715C85B"}},"block":{"header":{"version":{"block":"11"},"chain_id":"cube_47-5","height":"1359533","time":"2022-11-30T23:04:14.238581694Z","last_block_id":{"hash":"7E2F61B7151FEB1D75AB7B0AC6CA6B5CCAF4F2E1A357A0628E81685DD5D883B4","parts":{"total":1,"hash":"643E2069080D49C0E8FDB415D4A6841663FDB75614AB394CC5F48886A7ECE0FB"}},"last_commit_hash":"635D784E1B9CE60FC61404BC922FAF8D9681515620E1F1898D510232460488EB","data_hash":"571E95DBFFAAE05CDA0DF50CEB8189B3047BE06B4FDC0586B7153296FACD1871","validators_hash":"62C3ECD604F1CB9BC074CB08A37BBEA1EDD2FF5F228FF8B37AB3DD76D347C0B0","next_validators_hash":"62C3ECD604F1CB9BC074CB08A37BBEA1EDD2FF5F228FF8B37AB3DD76D347C0B0","consensus_hash":"E660EF14A95143DB0F3EAF2F31F177DE334DE5AB650579FD093A10CBAE86D5A6","app_hash":"6F5D472CE5798A7EAFA07B2147AAAADA550C23C58329EF3DB681FCDDD9CABB27","last_results_hash":"E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855","evidence_hash":"E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855","proposer_address":"3130821CF2DAA1C00BA599C4D05C51D54ACE2B9E"},"data":{"txs":["CpgBCpUBChwvY29zbW9zLmJhbmsudjFiZXRhMS5Nc2dTZW5kEnUKK3hwbGExaGR2ZjZ2djVhbWM3d3A4NGpzMGxzMjdhcGVrd3hwcjBnZTk2a2cSK3hwbGExYThmM3dubjdxd3Z3ZHp4a2M5dzg0OWtmemhycjZnZHZ5NGM4d3YaGQoFYXhwbGESEDUwMDAwMDAwMDAwMDAwMDASfgpZCk8KKC9ldGhlcm1pbnQuY3J5cHRvLnYxLmV0aHNlY3AyNTZrMS5QdWJLZXkSIwohAreNsVEsIEpsORnscZlxzo7Xha4JRK0a7v6rJwPR5U0CEgQKAggBGAMSIQobCgVheHBsYRISMTEyMjAwMDAwMDAwMDAwMDAwELDbBhpAXfRBuPdq7Icu13by0Nn51fU61K2VAR3a6Pie1BHeMZRmRGjyh4r5oG+eHCv6Ga2X52wkZnb6iFUuvMn2Ugvvnw=="]},"evidence":{"evidence":[]},"last_commit":{"height":"1359532","round":0,"block_id":{"hash":"7E2F61B7151FEB1D75AB7B0AC6CA6B5CCAF4F2E1A357A0628E81685DD5D883B4","parts":{"total":1,"hash":"643E2069080D49C0E8FDB415D4A6841663FDB75614AB394CC5F48886A7ECE0FB"}},"signatures":[{"block_id_flag":2,"validator_address":"51C5DEC2A8C0D876D8D799A096F19563F2C1273B","timestamp":"2022-11-30T23:04:14.235350753Z","signature":"eYgn7Geq9Kc6hSzgZut/mOHQx0HHfj+oSvDwTIM8jJCVlAn+rqp6eod7olyVy+42hWUNxtq6HBr1/167k0n9Dg=="},{"block_id_flag":2,"validator_address":"ED7E171129F79AB4D770433AC4CD1E6D121B57D6","timestamp":"2022-11-30T23:04:14.239212645Z","signature":"cENBbaH1nHGhggld9OANPkVwMQpSGcukUDtgQt5J3jCTqB7fTFKd5QJD/FwC3yqNvQJpbljVJj3Ge3sfHn2cCQ=="},{"block_id_flag":2,"validator_address":"A3BE52665F43F5A3200A4BEF6C670C978226B36F","timestamp":"2022-11-30T23:04:14.238581694Z","signature":"Acgz9dx86s1XnHuBz+zMJF6g5oJ92PvE0pzl/d7tLdg3x8dfnYj5Zt/PJ1/NCD3Gd3fj6OONlZiaESkKhheLCA=="},{"block_id_flag":2,"validator_address":"4F58BE62FB31F82BCAE53127A8ED030E5FE870FE","timestamp":"2022-11-30T23:04:14.238844376Z","signature":"D4+SsLDkCWuBzeRlDuRpZxnsLJFTDbxkI1QrlscJZmon8eQPvoiecifqbPm5blD4LH4De8AgrANAINMZyQkpCg=="},{"block_id_flag":2,"validator_address":"243FEF5563CA54682C1A187A83BB2EE5F1F24EC9","timestamp":"2022-11-30T23:04:14.138341689Z","signature":"jlbLJAY/BH41yg9cEUgzvVLcQYJYorw8zRN8HOnfP9UVYkrmWTXMw0WgJQw8jvesp/Sq0xsDaWHuJrRoGD7RCg=="},{"block_id_flag":2,"validator_address":"F3C67A5C47642658D97FD6110CE8326062A467D7","timestamp":"2022-11-30T23:04:14.138077895Z","signature":"x99Ywgy2t2+NrLwWBcB3wevhckJ08V6wl6YAPNdVSvggxxt9+eoQfmNIRGECw4t02g9elf7kE2cdxO2eo/ctAg=="},{"block_id_flag":2,"validator_address":"3130821CF2DAA1C00BA599C4D05C51D54ACE2B9E","timestamp":"2022-11-30T23:04:14.286272091Z","signature":"gboXioq/TwGiUG4Bkg0XklGLbEUoGl+Dnf4UTjfsAZkxBaR+pRMj4GUFW23jBPis7vIpNoUriOqPEgSqkg8tDw=="}]}}}}`,
				// abci_info
				`{"jsonrpc":"2.0","id":1,"result":{"response":{"data":"Xpla","version":"v1.1.2-cube","last_block_height":"1359640","last_block_app_hash":"wCZpDOY0V6x0WXmcW+P7kUTD3DJpZatwEdRyrgDZaK0="}}}`,
			},
			txinfo.LegacyTxInfo{
				TxID:            "7a13cb946589d07834119e3d9f3bf27e38da9990894e24850323582a404de46b",
				From:            "xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg",
				To:              "xpla1a8f3wnn7qwvwdzxkc9w849kfzhrr6gdvy4c8wv",
				ToAlt:           "",
				ContractAddress: "XPLA",
				Amount:          xc.NewAmountBlockchainFromUint64(5000000000000000),
				Fee:             xc.NewAmountBlockchainFromUint64(112200000000000000),
				FeePayer:        "xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg",
				BlockIndex:      1359533,
				BlockTime:       1669849454,
				Confirmations:   107,
				Status:          0,
				Sources: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address:         "xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg",
						ContractAddress: "XPLA",
						ContractId:      "axpla",
						Amount:          xc.NewAmountBlockchainFromUint64(5000000000000000),
						Event:           txinfo.NewEventFromIndex(10, txinfo.MovementVariantNative),
					},
				},
				Destinations: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address:         "xpla1a8f3wnn7qwvwdzxkc9w849kfzhrr6gdvy4c8wv",
						ContractAddress: "XPLA",
						ContractId:      "axpla",
						Amount:          xc.NewAmountBlockchainFromUint64(5000000000000000),
						Event:           txinfo.NewEventFromIndex(10, txinfo.MovementVariantNative),
					},
				},
			},
			"",
		},
		{
			// multi-Cw20 deposit on XPLA
			xc.NewChainConfig(xc.XPLA).WithChainCoin("axpla").WithChainPrefix("xpla"),
			"2C5A473586E23BEC60A92CE81AD36D7E7D5F09437B370C61C3F44CB5562FFB7F",
			[]string{
				// tx
				`{"id":0,"jsonrpc":"2.0","result":{"hash":"2C5A473586E23BEC60A92CE81AD36D7E7D5F09437B370C61C3F44CB5562FFB7F","height":"4855691","index":0,"tx":"Ct8JCvUBCiQvY29zbXdhc20ud2FzbS52MS5Nc2dFeGVjdXRlQ29udHJhY3QSzAEKK3hwbGExY2R0eTAzZnpxenFwa3ZmNHpwbXBsOXJubGZmamVleTdmYTVuNDcSP3hwbGExaHozc3ZnZGhtdjY3bHNxbGR1dTB0Y25kM2Y3NWMweHIwbXU0OGw2eXd1d2x6NDN6c3NqcWMwejJoNBpceyJ0cmFuc2ZlciI6eyJyZWNpcGllbnQiOiJ4cGxhMWVyeHp0MGNlZ2RxdHZyaHV1YWRocTZ5ZWFlbmtrbXN2OGRlMnJhIiwiYW1vdW50IjoiMTg0ODAzMDQifX0K8wEKJC9jb3Ntd2FzbS53YXNtLnYxLk1zZ0V4ZWN1dGVDb250cmFjdBLKAQoreHBsYTFjZHR5MDNmenF6cXBrdmY0enBtcGw5cm5sZmZqZWV5N2ZhNW40NxI/eHBsYTFoejNzdmdkaG12Njdsc3FsZHV1MHRjbmQzZjc1YzB4cjBtdTQ4bDZ5d3V3bHo0M3pzc2pxYzB6Mmg0Glp7InRyYW5zZmVyIjp7InJlY2lwaWVudCI6InhwbGExbTdlNHl5MmtyOHk5ZWZkZzM2eTJoc3U1ZXR5dWNqMjIyeXd5NmoiLCJhbW91bnQiOiIxOTA1MTkifX0K8wEKJC9jb3Ntd2FzbS53YXNtLnYxLk1zZ0V4ZWN1dGVDb250cmFjdBLKAQoreHBsYTFjZHR5MDNmenF6cXBrdmY0enBtcGw5cm5sZmZqZWV5N2ZhNW40NxI/eHBsYTFoejNzdmdkaG12Njdsc3FsZHV1MHRjbmQzZjc1YzB4cjBtdTQ4bDZ5d3V3bHo0M3pzc2pxYzB6Mmg0Glp7InRyYW5zZmVyIjp7InJlY2lwaWVudCI6InhwbGExcTR4bnM3ZXUzejh1M2FjajRuZDh6YTNuejN4dno3d2NtNzNjcDAiLCJhbW91bnQiOiIxOTA1MTkifX0K8gEKJC9jb3Ntd2FzbS53YXNtLnYxLk1zZ0V4ZWN1dGVDb250cmFjdBLJAQoreHBsYTFjZHR5MDNmenF6cXBrdmY0enBtcGw5cm5sZmZqZWV5N2ZhNW40NxI/eHBsYTFoejNzdmdkaG12Njdsc3FsZHV1MHRjbmQzZjc1YzB4cjBtdTQ4bDZ5d3V3bHo0M3pzc2pxYzB6Mmg0Gll7InRyYW5zZmVyIjp7InJlY2lwaWVudCI6InhwbGExMnh4cnlsamp4YXJ3Z3ljamVqZjdzc3J1cGFqNzhubXE4a2NjejYiLCJhbW91bnQiOiI5NTI2MCJ9fQryAQokL2Nvc213YXNtLndhc20udjEuTXNnRXhlY3V0ZUNvbnRyYWN0EskBCit4cGxhMWNkdHkwM2Z6cXpxcGt2ZjR6cG1wbDlybmxmZmplZXk3ZmE1bjQ3Ej94cGxhMWh6M3N2Z2RobXY2N2xzcWxkdXUwdGNuZDNmNzVjMHhyMG11NDhsNnl3dXdsejQzenNzanFjMHoyaDQaWXsidHJhbnNmZXIiOnsicmVjaXBpZW50IjoieHBsYTFnOGhremtnZmEzdXEwY2c5ZDZoOTlqazVubGc5Mmx3eDJqbWUybCIsImFtb3VudCI6Ijk1MjYwIn19Eg8xMjkzMzMtZGVhcmVsbGESiQIKWwpPCigvZXRoZXJtaW50LmNyeXB0by52MS5ldGhzZWNwMjU2azEuUHViS2V5EiMKIQOP1oiUFpB9fApsI4VSKjRawzC4rkbqRLVMwTW1pyah5RIECgIIfxiLhwcKWgpPCigvZXRoZXJtaW50LmNyeXB0by52MS5ldGhzZWNwMjU2azEuUHViS2V5EiMKIQITgb/sp0g+uq9LuT4/TFxv6XC6s+tCn1Luhl9lxdQrAxIECgIIfxiAARJOChsKBWF4cGxhEhI1ODM2MTA4NTAwMDAwMDAwMDAQifQpGit4cGxhMWVyeHp0MGNlZ2RxdHZyaHV1YWRocTZ5ZWFlbmtrbXN2OGRlMnJhGkAacRusa9HghwoSJBuriXAhGYRZAPLnoSAbGsQf5cwG4QqmQLTdKucbbyAtm9c2A+dmsbnh7R9jBAOVTax0v8wBGkBlx9JHC1fjSC5PI6eVxG24Iumai18fJ0kWTppuqLFO6BIdeoLpXzD3ygeEPGzYNkPl9VwEOjGXeGabACiiaPUD","tx_result":{"code":0,"codespace":"","data":"CiYKJC9jb3Ntd2FzbS53YXNtLnYxLk1zZ0V4ZWN1dGVDb250cmFjdAomCiQvY29zbXdhc20ud2FzbS52MS5Nc2dFeGVjdXRlQ29udHJhY3QKJgokL2Nvc213YXNtLndhc20udjEuTXNnRXhlY3V0ZUNvbnRyYWN0CiYKJC9jb3Ntd2FzbS53YXNtLnYxLk1zZ0V4ZWN1dGVDb250cmFjdAomCiQvY29zbXdhc20ud2FzbS52MS5Nc2dFeGVjdXRlQ29udHJhY3Q=","events":[{"attributes":[{"index":true,"key":"c3BlbmRlcg==","value":"eHBsYTFlcnh6dDBjZWdkcXR2cmh1dWFkaHE2eWVhZW5ra21zdjhkZTJyYQ=="},{"index":true,"key":"YW1vdW50","value":"NTgzNjEwODUwMDAwMDAwMDAwYXhwbGE="}],"type":"coin_spent"},{"attributes":[{"index":true,"key":"cmVjZWl2ZXI=","value":"eHBsYTE3eHBmdmFrbTJhbWc5NjJ5bHM2Zjg0ejNrZWxsOGM1bHc3bXVxdw=="},{"index":true,"key":"YW1vdW50","value":"NTgzNjEwODUwMDAwMDAwMDAwYXhwbGE="}],"type":"coin_received"},{"attributes":[{"index":true,"key":"cmVjaXBpZW50","value":"eHBsYTE3eHBmdmFrbTJhbWc5NjJ5bHM2Zjg0ejNrZWxsOGM1bHc3bXVxdw=="},{"index":true,"key":"c2VuZGVy","value":"eHBsYTFlcnh6dDBjZWdkcXR2cmh1dWFkaHE2eWVhZW5ra21zdjhkZTJyYQ=="},{"index":true,"key":"YW1vdW50","value":"NTgzNjEwODUwMDAwMDAwMDAwYXhwbGE="}],"type":"transfer"},{"attributes":[{"index":true,"key":"c2VuZGVy","value":"eHBsYTFlcnh6dDBjZWdkcXR2cmh1dWFkaHE2eWVhZW5ra21zdjhkZTJyYQ=="}],"type":"message"},{"attributes":[{"index":true,"key":"ZmVl","value":"NTgzNjEwODUwMDAwMDAwMDAwYXhwbGE="},{"index":true,"key":"ZmVlX3BheWVy","value":"eHBsYTFlcnh6dDBjZWdkcXR2cmh1dWFkaHE2eWVhZW5ra21zdjhkZTJyYQ=="}],"type":"tx"},{"attributes":[{"index":true,"key":"YWNjX3NlcQ==","value":"eHBsYTFjZHR5MDNmenF6cXBrdmY0enBtcGw5cm5sZmZqZWV5N2ZhNW40Ny8xMTU1OTU="}],"type":"tx"},{"attributes":[{"index":true,"key":"c2lnbmF0dXJl","value":"R25FYnJHdlI0SWNLRWlRYnE0bHdJUm1FV1FEeTU2RWdHeHJFSCtYTUJ1RUtwa0MwM1NybkcyOGdMWnZYTmdQblpyRzU0ZTBmWXdRRGxVMnNkTC9NQVE9PQ=="}],"type":"tx"},{"attributes":[{"index":true,"key":"YWNjX3NlcQ==","value":"eHBsYTFlcnh6dDBjZWdkcXR2cmh1dWFkaHE2eWVhZW5ra21zdjhkZTJyYS8xMjg="}],"type":"tx"},{"attributes":[{"index":true,"key":"c2lnbmF0dXJl","value":"WmNmU1J3dFg0MGd1VHlPbmxjUnR1Q0xwbW90Zkh5ZEpGazZhYnFpeFR1Z1NIWHFDNlY4dzk4b0hoRHhzMkRaRDVmVmNCRG94bDNobW13QW9vbWoxQXc9PQ=="}],"type":"tx"},{"attributes":[{"index":true,"key":"YWN0aW9u","value":"L2Nvc213YXNtLndhc20udjEuTXNnRXhlY3V0ZUNvbnRyYWN0"}],"type":"message"},{"attributes":[{"index":true,"key":"bW9kdWxl","value":"d2FzbQ=="},{"index":true,"key":"c2VuZGVy","value":"eHBsYTFjZHR5MDNmenF6cXBrdmY0enBtcGw5cm5sZmZqZWV5N2ZhNW40Nw=="}],"type":"message"},{"attributes":[{"index":true,"key":"X2NvbnRyYWN0X2FkZHJlc3M=","value":"eHBsYTFoejNzdmdkaG12Njdsc3FsZHV1MHRjbmQzZjc1YzB4cjBtdTQ4bDZ5d3V3bHo0M3pzc2pxYzB6Mmg0"}],"type":"execute"},{"attributes":[{"index":true,"key":"X2NvbnRyYWN0X2FkZHJlc3M=","value":"eHBsYTFoejNzdmdkaG12Njdsc3FsZHV1MHRjbmQzZjc1YzB4cjBtdTQ4bDZ5d3V3bHo0M3pzc2pxYzB6Mmg0"},{"index":true,"key":"YWN0aW9u","value":"dHJhbnNmZXI="},{"index":true,"key":"ZnJvbQ==","value":"eHBsYTFjZHR5MDNmenF6cXBrdmY0enBtcGw5cm5sZmZqZWV5N2ZhNW40Nw=="},{"index":true,"key":"dG8=","value":"eHBsYTFlcnh6dDBjZWdkcXR2cmh1dWFkaHE2eWVhZW5ra21zdjhkZTJyYQ=="},{"index":true,"key":"YW1vdW50","value":"MTg0ODAzMDQ="}],"type":"wasm"},{"attributes":[{"index":true,"key":"YWN0aW9u","value":"L2Nvc213YXNtLndhc20udjEuTXNnRXhlY3V0ZUNvbnRyYWN0"}],"type":"message"},{"attributes":[{"index":true,"key":"bW9kdWxl","value":"d2FzbQ=="},{"index":true,"key":"c2VuZGVy","value":"eHBsYTFjZHR5MDNmenF6cXBrdmY0enBtcGw5cm5sZmZqZWV5N2ZhNW40Nw=="}],"type":"message"},{"attributes":[{"index":true,"key":"X2NvbnRyYWN0X2FkZHJlc3M=","value":"eHBsYTFoejNzdmdkaG12Njdsc3FsZHV1MHRjbmQzZjc1YzB4cjBtdTQ4bDZ5d3V3bHo0M3pzc2pxYzB6Mmg0"}],"type":"execute"},{"attributes":[{"index":true,"key":"X2NvbnRyYWN0X2FkZHJlc3M=","value":"eHBsYTFoejNzdmdkaG12Njdsc3FsZHV1MHRjbmQzZjc1YzB4cjBtdTQ4bDZ5d3V3bHo0M3pzc2pxYzB6Mmg0"},{"index":true,"key":"YWN0aW9u","value":"dHJhbnNmZXI="},{"index":true,"key":"ZnJvbQ==","value":"eHBsYTFjZHR5MDNmenF6cXBrdmY0enBtcGw5cm5sZmZqZWV5N2ZhNW40Nw=="},{"index":true,"key":"dG8=","value":"eHBsYTFtN2U0eXkya3I4eTllZmRnMzZ5MmhzdTVldHl1Y2oyMjJ5d3k2ag=="},{"index":true,"key":"YW1vdW50","value":"MTkwNTE5"}],"type":"wasm"},{"attributes":[{"index":true,"key":"YWN0aW9u","value":"L2Nvc213YXNtLndhc20udjEuTXNnRXhlY3V0ZUNvbnRyYWN0"}],"type":"message"},{"attributes":[{"index":true,"key":"bW9kdWxl","value":"d2FzbQ=="},{"index":true,"key":"c2VuZGVy","value":"eHBsYTFjZHR5MDNmenF6cXBrdmY0enBtcGw5cm5sZmZqZWV5N2ZhNW40Nw=="}],"type":"message"},{"attributes":[{"index":true,"key":"X2NvbnRyYWN0X2FkZHJlc3M=","value":"eHBsYTFoejNzdmdkaG12Njdsc3FsZHV1MHRjbmQzZjc1YzB4cjBtdTQ4bDZ5d3V3bHo0M3pzc2pxYzB6Mmg0"}],"type":"execute"},{"attributes":[{"index":true,"key":"X2NvbnRyYWN0X2FkZHJlc3M=","value":"eHBsYTFoejNzdmdkaG12Njdsc3FsZHV1MHRjbmQzZjc1YzB4cjBtdTQ4bDZ5d3V3bHo0M3pzc2pxYzB6Mmg0"},{"index":true,"key":"YWN0aW9u","value":"dHJhbnNmZXI="},{"index":true,"key":"ZnJvbQ==","value":"eHBsYTFjZHR5MDNmenF6cXBrdmY0enBtcGw5cm5sZmZqZWV5N2ZhNW40Nw=="},{"index":true,"key":"dG8=","value":"eHBsYTFxNHhuczdldTN6OHUzYWNqNG5kOHphM256M3h2ejd3Y203M2NwMA=="},{"index":true,"key":"YW1vdW50","value":"MTkwNTE5"}],"type":"wasm"},{"attributes":[{"index":true,"key":"YWN0aW9u","value":"L2Nvc213YXNtLndhc20udjEuTXNnRXhlY3V0ZUNvbnRyYWN0"}],"type":"message"},{"attributes":[{"index":true,"key":"bW9kdWxl","value":"d2FzbQ=="},{"index":true,"key":"c2VuZGVy","value":"eHBsYTFjZHR5MDNmenF6cXBrdmY0enBtcGw5cm5sZmZqZWV5N2ZhNW40Nw=="}],"type":"message"},{"attributes":[{"index":true,"key":"X2NvbnRyYWN0X2FkZHJlc3M=","value":"eHBsYTFoejNzdmdkaG12Njdsc3FsZHV1MHRjbmQzZjc1YzB4cjBtdTQ4bDZ5d3V3bHo0M3pzc2pxYzB6Mmg0"}],"type":"execute"},{"attributes":[{"index":true,"key":"X2NvbnRyYWN0X2FkZHJlc3M=","value":"eHBsYTFoejNzdmdkaG12Njdsc3FsZHV1MHRjbmQzZjc1YzB4cjBtdTQ4bDZ5d3V3bHo0M3pzc2pxYzB6Mmg0"},{"index":true,"key":"YWN0aW9u","value":"dHJhbnNmZXI="},{"index":true,"key":"ZnJvbQ==","value":"eHBsYTFjZHR5MDNmenF6cXBrdmY0enBtcGw5cm5sZmZqZWV5N2ZhNW40Nw=="},{"index":true,"key":"dG8=","value":"eHBsYTEyeHhyeWxqanhhcndneWNqZWpmN3NzcnVwYWo3OG5tcThrY2N6Ng=="},{"index":true,"key":"YW1vdW50","value":"OTUyNjA="}],"type":"wasm"},{"attributes":[{"index":true,"key":"YWN0aW9u","value":"L2Nvc213YXNtLndhc20udjEuTXNnRXhlY3V0ZUNvbnRyYWN0"}],"type":"message"},{"attributes":[{"index":true,"key":"bW9kdWxl","value":"d2FzbQ=="},{"index":true,"key":"c2VuZGVy","value":"eHBsYTFjZHR5MDNmenF6cXBrdmY0enBtcGw5cm5sZmZqZWV5N2ZhNW40Nw=="}],"type":"message"},{"attributes":[{"index":true,"key":"X2NvbnRyYWN0X2FkZHJlc3M=","value":"eHBsYTFoejNzdmdkaG12Njdsc3FsZHV1MHRjbmQzZjc1YzB4cjBtdTQ4bDZ5d3V3bHo0M3pzc2pxYzB6Mmg0"}],"type":"execute"},{"attributes":[{"index":true,"key":"X2NvbnRyYWN0X2FkZHJlc3M=","value":"eHBsYTFoejNzdmdkaG12Njdsc3FsZHV1MHRjbmQzZjc1YzB4cjBtdTQ4bDZ5d3V3bHo0M3pzc2pxYzB6Mmg0"},{"index":true,"key":"YWN0aW9u","value":"dHJhbnNmZXI="},{"index":true,"key":"ZnJvbQ==","value":"eHBsYTFjZHR5MDNmenF6cXBrdmY0enBtcGw5cm5sZmZqZWV5N2ZhNW40Nw=="},{"index":true,"key":"dG8=","value":"eHBsYTFnOGhremtnZmEzdXEwY2c5ZDZoOTlqazVubGc5Mmx3eDJqbWUybA=="},{"index":true,"key":"YW1vdW50","value":"OTUyNjA="}],"type":"wasm"}],"gas_used":"513369","gas_wanted":"686601","info":"","log":"[{\"events\":[{\"type\":\"execute\",\"attributes\":[{\"key\":\"_contract_address\",\"value\":\"xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4\"}]},{\"type\":\"message\",\"attributes\":[{\"key\":\"action\",\"value\":\"/cosmwasm.wasm.v1.MsgExecuteContract\"},{\"key\":\"module\",\"value\":\"wasm\"},{\"key\":\"sender\",\"value\":\"xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47\"}]},{\"type\":\"wasm\",\"attributes\":[{\"key\":\"_contract_address\",\"value\":\"xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4\"},{\"key\":\"action\",\"value\":\"transfer\"},{\"key\":\"from\",\"value\":\"xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47\"},{\"key\":\"to\",\"value\":\"xpla1erxzt0cegdqtvrhuuadhq6yeaenkkmsv8de2ra\"},{\"key\":\"amount\",\"value\":\"18480304\"}]}]},{\"msg_index\":1,\"events\":[{\"type\":\"execute\",\"attributes\":[{\"key\":\"_contract_address\",\"value\":\"xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4\"}]},{\"type\":\"message\",\"attributes\":[{\"key\":\"action\",\"value\":\"/cosmwasm.wasm.v1.MsgExecuteContract\"},{\"key\":\"module\",\"value\":\"wasm\"},{\"key\":\"sender\",\"value\":\"xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47\"}]},{\"type\":\"wasm\",\"attributes\":[{\"key\":\"_contract_address\",\"value\":\"xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4\"},{\"key\":\"action\",\"value\":\"transfer\"},{\"key\":\"from\",\"value\":\"xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47\"},{\"key\":\"to\",\"value\":\"xpla1m7e4yy2kr8y9efdg36y2hsu5etyucj222ywy6j\"},{\"key\":\"amount\",\"value\":\"190519\"}]}]},{\"msg_index\":2,\"events\":[{\"type\":\"execute\",\"attributes\":[{\"key\":\"_contract_address\",\"value\":\"xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4\"}]},{\"type\":\"message\",\"attributes\":[{\"key\":\"action\",\"value\":\"/cosmwasm.wasm.v1.MsgExecuteContract\"},{\"key\":\"module\",\"value\":\"wasm\"},{\"key\":\"sender\",\"value\":\"xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47\"}]},{\"type\":\"wasm\",\"attributes\":[{\"key\":\"_contract_address\",\"value\":\"xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4\"},{\"key\":\"action\",\"value\":\"transfer\"},{\"key\":\"from\",\"value\":\"xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47\"},{\"key\":\"to\",\"value\":\"xpla1q4xns7eu3z8u3acj4nd8za3nz3xvz7wcm73cp0\"},{\"key\":\"amount\",\"value\":\"190519\"}]}]},{\"msg_index\":3,\"events\":[{\"type\":\"execute\",\"attributes\":[{\"key\":\"_contract_address\",\"value\":\"xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4\"}]},{\"type\":\"message\",\"attributes\":[{\"key\":\"action\",\"value\":\"/cosmwasm.wasm.v1.MsgExecuteContract\"},{\"key\":\"module\",\"value\":\"wasm\"},{\"key\":\"sender\",\"value\":\"xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47\"}]},{\"type\":\"wasm\",\"attributes\":[{\"key\":\"_contract_address\",\"value\":\"xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4\"},{\"key\":\"action\",\"value\":\"transfer\"},{\"key\":\"from\",\"value\":\"xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47\"},{\"key\":\"to\",\"value\":\"xpla12xxryljjxarwgycjejf7ssrupaj78nmq8kccz6\"},{\"key\":\"amount\",\"value\":\"95260\"}]}]},{\"msg_index\":4,\"events\":[{\"type\":\"execute\",\"attributes\":[{\"key\":\"_contract_address\",\"value\":\"xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4\"}]},{\"type\":\"message\",\"attributes\":[{\"key\":\"action\",\"value\":\"/cosmwasm.wasm.v1.MsgExecuteContract\"},{\"key\":\"module\",\"value\":\"wasm\"},{\"key\":\"sender\",\"value\":\"xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47\"}]},{\"type\":\"wasm\",\"attributes\":[{\"key\":\"_contract_address\",\"value\":\"xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4\"},{\"key\":\"action\",\"value\":\"transfer\"},{\"key\":\"from\",\"value\":\"xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47\"},{\"key\":\"to\",\"value\":\"xpla1g8hkzkgfa3uq0cg9d6h99jk5nlg92lwx2jme2l\"},{\"key\":\"amount\",\"value\":\"95260\"}]}]}]"}}}`,
				// block
				`{"jsonrpc":"2.0","id":0,"result":{"block_id":{"hash":"9A1F181DF132ECF61AFC674063178DF42264C302EF78F33DBD5E83754ED30D4C","parts":{"total":1,"hash":"3470448F8E010EADB949CD81CD5DA5A04C55502FD856C60E38B8FA2CE715C85B"}},"block":{"header":{"version":{"block":"11"},"chain_id":"cube_47-5","height":"4855889","time":"2022-11-30T23:04:14.238581694Z","last_block_id":{"hash":"7E2F61B7151FEB1D75AB7B0AC6CA6B5CCAF4F2E1A357A0628E81685DD5D883B4","parts":{"total":1,"hash":"643E2069080D49C0E8FDB415D4A6841663FDB75614AB394CC5F48886A7ECE0FB"}},"last_commit_hash":"635D784E1B9CE60FC61404BC922FAF8D9681515620E1F1898D510232460488EB","data_hash":"571E95DBFFAAE05CDA0DF50CEB8189B3047BE06B4FDC0586B7153296FACD1871","validators_hash":"62C3ECD604F1CB9BC074CB08A37BBEA1EDD2FF5F228FF8B37AB3DD76D347C0B0","next_validators_hash":"62C3ECD604F1CB9BC074CB08A37BBEA1EDD2FF5F228FF8B37AB3DD76D347C0B0","consensus_hash":"E660EF14A95143DB0F3EAF2F31F177DE334DE5AB650579FD093A10CBAE86D5A6","app_hash":"6F5D472CE5798A7EAFA07B2147AAAADA550C23C58329EF3DB681FCDDD9CABB27","last_results_hash":"E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855","evidence_hash":"E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855","proposer_address":"3130821CF2DAA1C00BA599C4D05C51D54ACE2B9E"},"data":{"txs":["CpgBCpUBChwvY29zbW9zLmJhbmsudjFiZXRhMS5Nc2dTZW5kEnUKK3hwbGExaGR2ZjZ2djVhbWM3d3A4NGpzMGxzMjdhcGVrd3hwcjBnZTk2a2cSK3hwbGExYThmM3dubjdxd3Z3ZHp4a2M5dzg0OWtmemhycjZnZHZ5NGM4d3YaGQoFYXhwbGESEDUwMDAwMDAwMDAwMDAwMDASfgpZCk8KKC9ldGhlcm1pbnQuY3J5cHRvLnYxLmV0aHNlY3AyNTZrMS5QdWJLZXkSIwohAreNsVEsIEpsORnscZlxzo7Xha4JRK0a7v6rJwPR5U0CEgQKAggBGAMSIQobCgVheHBsYRISMTEyMjAwMDAwMDAwMDAwMDAwELDbBhpAXfRBuPdq7Icu13by0Nn51fU61K2VAR3a6Pie1BHeMZRmRGjyh4r5oG+eHCv6Ga2X52wkZnb6iFUuvMn2Ugvvnw=="]},"evidence":{"evidence":[]},"last_commit":{"height":"1359532","round":0,"block_id":{"hash":"7E2F61B7151FEB1D75AB7B0AC6CA6B5CCAF4F2E1A357A0628E81685DD5D883B4","parts":{"total":1,"hash":"643E2069080D49C0E8FDB415D4A6841663FDB75614AB394CC5F48886A7ECE0FB"}},"signatures":[{"block_id_flag":2,"validator_address":"51C5DEC2A8C0D876D8D799A096F19563F2C1273B","timestamp":"2022-11-30T23:04:14.235350753Z","signature":"eYgn7Geq9Kc6hSzgZut/mOHQx0HHfj+oSvDwTIM8jJCVlAn+rqp6eod7olyVy+42hWUNxtq6HBr1/167k0n9Dg=="},{"block_id_flag":2,"validator_address":"ED7E171129F79AB4D770433AC4CD1E6D121B57D6","timestamp":"2022-11-30T23:04:14.239212645Z","signature":"cENBbaH1nHGhggld9OANPkVwMQpSGcukUDtgQt5J3jCTqB7fTFKd5QJD/FwC3yqNvQJpbljVJj3Ge3sfHn2cCQ=="},{"block_id_flag":2,"validator_address":"A3BE52665F43F5A3200A4BEF6C670C978226B36F","timestamp":"2022-11-30T23:04:14.238581694Z","signature":"Acgz9dx86s1XnHuBz+zMJF6g5oJ92PvE0pzl/d7tLdg3x8dfnYj5Zt/PJ1/NCD3Gd3fj6OONlZiaESkKhheLCA=="},{"block_id_flag":2,"validator_address":"4F58BE62FB31F82BCAE53127A8ED030E5FE870FE","timestamp":"2022-11-30T23:04:14.238844376Z","signature":"D4+SsLDkCWuBzeRlDuRpZxnsLJFTDbxkI1QrlscJZmon8eQPvoiecifqbPm5blD4LH4De8AgrANAINMZyQkpCg=="},{"block_id_flag":2,"validator_address":"243FEF5563CA54682C1A187A83BB2EE5F1F24EC9","timestamp":"2022-11-30T23:04:14.138341689Z","signature":"jlbLJAY/BH41yg9cEUgzvVLcQYJYorw8zRN8HOnfP9UVYkrmWTXMw0WgJQw8jvesp/Sq0xsDaWHuJrRoGD7RCg=="},{"block_id_flag":2,"validator_address":"F3C67A5C47642658D97FD6110CE8326062A467D7","timestamp":"2022-11-30T23:04:14.138077895Z","signature":"x99Ywgy2t2+NrLwWBcB3wevhckJ08V6wl6YAPNdVSvggxxt9+eoQfmNIRGECw4t02g9elf7kE2cdxO2eo/ctAg=="},{"block_id_flag":2,"validator_address":"3130821CF2DAA1C00BA599C4D05C51D54ACE2B9E","timestamp":"2022-11-30T23:04:14.286272091Z","signature":"gboXioq/TwGiUG4Bkg0XklGLbEUoGl+Dnf4UTjfsAZkxBaR+pRMj4GUFW23jBPis7vIpNoUriOqPEgSqkg8tDw=="}]}}}}`,
				// abci_info
				`{"jsonrpc":"2.0","id":1,"result":{"response":{"data":"Xpla","version":"v1.1.2-cube","last_block_height":"4855889","last_block_app_hash":"wCZpDOY0V6x0WXmcW+P7kUTD3DJpZatwEdRyrgDZaK0="}}}`,
			},
			txinfo.LegacyTxInfo{
				TxID:            "2C5A473586E23BEC60A92CE81AD36D7E7D5F09437B370C61C3F44CB5562FFB7F",
				From:            "xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47",
				To:              "xpla1erxzt0cegdqtvrhuuadhq6yeaenkkmsv8de2ra",
				ToAlt:           "",
				ContractAddress: "xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4",
				Amount:          xc.NewAmountBlockchainFromUint64(18480304),
				Fee:             xc.NewAmountBlockchainFromUint64(583610850000000000),
				FeePayer:        "xpla1erxzt0cegdqtvrhuuadhq6yeaenkkmsv8de2ra",
				BlockIndex:      4855691,
				BlockTime:       1669849454,
				Confirmations:   198,
				Status:          0,
				Sources: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address:         "xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47",
						ContractAddress: "xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4",
						Amount:          xc.NewAmountBlockchainFromUint64(18480304),
						Event:           txinfo.NewEventFromIndex(12, txinfo.MovementVariantNative),
					},
					{
						Address:         "xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47",
						ContractAddress: "xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4",
						Amount:          xc.NewAmountBlockchainFromUint64(190519),
						Event:           txinfo.NewEventFromIndex(16, txinfo.MovementVariantNative),
					},
					{
						Address:         "xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47",
						ContractAddress: "xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4",
						Amount:          xc.NewAmountBlockchainFromUint64(190519),
						Event:           txinfo.NewEventFromIndex(20, txinfo.MovementVariantNative),
					},
					{
						Address:         "xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47",
						ContractAddress: "xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4",
						Amount:          xc.NewAmountBlockchainFromUint64(95260),
						Event:           txinfo.NewEventFromIndex(24, txinfo.MovementVariantNative),
					},
					{
						Address:         "xpla1cdty03fzqzqpkvf4zpmpl9rnlffjeey7fa5n47",
						ContractAddress: "xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4",
						Amount:          xc.NewAmountBlockchainFromUint64(95260),
						Event:           txinfo.NewEventFromIndex(28, txinfo.MovementVariantNative),
					},
				},
				Destinations: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address:         "xpla1erxzt0cegdqtvrhuuadhq6yeaenkkmsv8de2ra",
						ContractAddress: "xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4",
						Amount:          xc.NewAmountBlockchainFromUint64(18480304),
						Memo:            "129333-dearella",
						Event:           txinfo.NewEventFromIndex(12, txinfo.MovementVariantNative),
					},
					{
						Address:         "xpla1m7e4yy2kr8y9efdg36y2hsu5etyucj222ywy6j",
						ContractAddress: "xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4",
						Amount:          xc.NewAmountBlockchainFromUint64(190519),
						Memo:            "129333-dearella",
						Event:           txinfo.NewEventFromIndex(16, txinfo.MovementVariantNative),
					},
					{
						Address:         "xpla1q4xns7eu3z8u3acj4nd8za3nz3xvz7wcm73cp0",
						ContractAddress: "xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4",
						Amount:          xc.NewAmountBlockchainFromUint64(190519),
						Memo:            "129333-dearella",
						Event:           txinfo.NewEventFromIndex(20, txinfo.MovementVariantNative),
					},
					{
						Address:         "xpla12xxryljjxarwgycjejf7ssrupaj78nmq8kccz6",
						ContractAddress: "xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4",
						Amount:          xc.NewAmountBlockchainFromUint64(95260),
						Memo:            "129333-dearella",
						Event:           txinfo.NewEventFromIndex(24, txinfo.MovementVariantNative),
					},
					{
						Address:         "xpla1g8hkzkgfa3uq0cg9d6h99jk5nlg92lwx2jme2l",
						ContractAddress: "xpla1hz3svgdhmv67lsqlduu0tcnd3f75c0xr0mu48l6ywuwlz43zssjqc0z2h4",
						Amount:          xc.NewAmountBlockchainFromUint64(95260),
						Memo:            "129333-dearella",
						Event:           txinfo.NewEventFromIndex(28, txinfo.MovementVariantNative),
					},
				},
			},
			"",
		},
		{
			// send XPLA but it reverts
			xc.NewChainConfig(xc.XPLA).WithChainCoin("axpla").WithChainPrefix("xpla"),
			"7a13cb946589d07834119e3d9f3bf27e38da9990894e24850323582a404de46b",
			[]string{
				// tx
				`{"jsonrpc":"2.0","id":0,"result":{"hash":"7A13CB946589D07834119E3D9F3BF27E38DA9990894E24850323582A404DE46B","height":"1359533","index":0,"tx_result":{"code":100,"data":"Ch4KHC9jb3Ntb3MuYmFuay52MWJldGExLk1zZ1NlbmQ=","log":"{\"error\": \"no money\"}","info":"","gas_wanted":"110000","gas_used":"93146","events":[{"type":"coin_spent","attributes":[{"key":"c3BlbmRlcg==","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZw==","index":true},{"key":"YW1vdW50","value":"MTEyMjAwMDAwMDAwMDAwMDAwYXhwbGE=","index":true}]},{"type":"coin_received","attributes":[{"key":"cmVjZWl2ZXI=","value":"eHBsYTE3eHBmdmFrbTJhbWc5NjJ5bHM2Zjg0ejNrZWxsOGM1bHc3bXVxdw==","index":true},{"key":"YW1vdW50","value":"MTEyMjAwMDAwMDAwMDAwMDAwYXhwbGE=","index":true}]},{"type":"transfer","attributes":[{"key":"cmVjaXBpZW50","value":"eHBsYTE3eHBmdmFrbTJhbWc5NjJ5bHM2Zjg0ejNrZWxsOGM1bHc3bXVxdw==","index":true},{"key":"c2VuZGVy","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZw==","index":true},{"key":"YW1vdW50","value":"MTEyMjAwMDAwMDAwMDAwMDAwYXhwbGE=","index":true}]},{"type":"message","attributes":[{"key":"c2VuZGVy","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZw==","index":true}]},{"type":"tx","attributes":[{"key":"ZmVl","value":"MTEyMjAwMDAwMDAwMDAwMDAwYXhwbGE=","index":true},{"key":"ZmVlX3BheWVy","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZw==","index":true}]},{"type":"tx","attributes":[{"key":"YWNjX3NlcQ==","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZy8z","index":true}]},{"type":"tx","attributes":[{"key":"c2lnbmF0dXJl","value":"WGZSQnVQZHE3SWN1MTNieTBObjUxZlU2MUsyVkFSM2E2UGllMUJIZU1aUm1SR2p5aDRyNW9HK2VIQ3Y2R2EyWDUyd2tabmI2aUZVdXZNbjJVZ3Z2bnc9PQ==","index":true}]},{"type":"message","attributes":[{"key":"YWN0aW9u","value":"L2Nvc21vcy5iYW5rLnYxYmV0YTEuTXNnU2VuZA==","index":true}]},{"type":"coin_spent","attributes":[{"key":"c3BlbmRlcg==","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZw==","index":true},{"key":"YW1vdW50","value":"NTAwMDAwMDAwMDAwMDAwMGF4cGxh","index":true}]},{"type":"coin_received","attributes":[{"key":"cmVjZWl2ZXI=","value":"eHBsYTFhOGYzd25uN3F3dndkenhrYzl3ODQ5a2Z6aHJyNmdkdnk0Yzh3dg==","index":true},{"key":"YW1vdW50","value":"NTAwMDAwMDAwMDAwMDAwMGF4cGxh","index":true}]},{"type":"transfer","attributes":[{"key":"cmVjaXBpZW50","value":"eHBsYTFhOGYzd25uN3F3dndkenhrYzl3ODQ5a2Z6aHJyNmdkdnk0Yzh3dg==","index":true},{"key":"c2VuZGVy","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZw==","index":true},{"key":"YW1vdW50","value":"NTAwMDAwMDAwMDAwMDAwMGF4cGxh","index":true}]},{"type":"message","attributes":[{"key":"c2VuZGVy","value":"eHBsYTFoZHZmNnZ2NWFtYzd3cDg0anMwbHMyN2FwZWt3eHByMGdlOTZrZw==","index":true}]},{"type":"message","attributes":[{"key":"bW9kdWxl","value":"YmFuaw==","index":true}]}],"codespace":""},"tx":"CpgBCpUBChwvY29zbW9zLmJhbmsudjFiZXRhMS5Nc2dTZW5kEnUKK3hwbGExaGR2ZjZ2djVhbWM3d3A4NGpzMGxzMjdhcGVrd3hwcjBnZTk2a2cSK3hwbGExYThmM3dubjdxd3Z3ZHp4a2M5dzg0OWtmemhycjZnZHZ5NGM4d3YaGQoFYXhwbGESEDUwMDAwMDAwMDAwMDAwMDASfgpZCk8KKC9ldGhlcm1pbnQuY3J5cHRvLnYxLmV0aHNlY3AyNTZrMS5QdWJLZXkSIwohAreNsVEsIEpsORnscZlxzo7Xha4JRK0a7v6rJwPR5U0CEgQKAggBGAMSIQobCgVheHBsYRISMTEyMjAwMDAwMDAwMDAwMDAwELDbBhpAXfRBuPdq7Icu13by0Nn51fU61K2VAR3a6Pie1BHeMZRmRGjyh4r5oG+eHCv6Ga2X52wkZnb6iFUuvMn2Ugvvnw=="}}`,
				// block
				`{"jsonrpc":"2.0","id":0,"result":{"block_id":{"hash":"9A1F181DF132ECF61AFC674063178DF42264C302EF78F33DBD5E83754ED30D4C","parts":{"total":1,"hash":"3470448F8E010EADB949CD81CD5DA5A04C55502FD856C60E38B8FA2CE715C85B"}},"block":{"header":{"version":{"block":"11"},"chain_id":"cube_47-5","height":"1359533","time":"2022-11-30T23:04:14.238581694Z","last_block_id":{"hash":"7E2F61B7151FEB1D75AB7B0AC6CA6B5CCAF4F2E1A357A0628E81685DD5D883B4","parts":{"total":1,"hash":"643E2069080D49C0E8FDB415D4A6841663FDB75614AB394CC5F48886A7ECE0FB"}},"last_commit_hash":"635D784E1B9CE60FC61404BC922FAF8D9681515620E1F1898D510232460488EB","data_hash":"571E95DBFFAAE05CDA0DF50CEB8189B3047BE06B4FDC0586B7153296FACD1871","validators_hash":"62C3ECD604F1CB9BC074CB08A37BBEA1EDD2FF5F228FF8B37AB3DD76D347C0B0","next_validators_hash":"62C3ECD604F1CB9BC074CB08A37BBEA1EDD2FF5F228FF8B37AB3DD76D347C0B0","consensus_hash":"E660EF14A95143DB0F3EAF2F31F177DE334DE5AB650579FD093A10CBAE86D5A6","app_hash":"6F5D472CE5798A7EAFA07B2147AAAADA550C23C58329EF3DB681FCDDD9CABB27","last_results_hash":"E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855","evidence_hash":"E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855","proposer_address":"3130821CF2DAA1C00BA599C4D05C51D54ACE2B9E"},"data":{"txs":["CpgBCpUBChwvY29zbW9zLmJhbmsudjFiZXRhMS5Nc2dTZW5kEnUKK3hwbGExaGR2ZjZ2djVhbWM3d3A4NGpzMGxzMjdhcGVrd3hwcjBnZTk2a2cSK3hwbGExYThmM3dubjdxd3Z3ZHp4a2M5dzg0OWtmemhycjZnZHZ5NGM4d3YaGQoFYXhwbGESEDUwMDAwMDAwMDAwMDAwMDASfgpZCk8KKC9ldGhlcm1pbnQuY3J5cHRvLnYxLmV0aHNlY3AyNTZrMS5QdWJLZXkSIwohAreNsVEsIEpsORnscZlxzo7Xha4JRK0a7v6rJwPR5U0CEgQKAggBGAMSIQobCgVheHBsYRISMTEyMjAwMDAwMDAwMDAwMDAwELDbBhpAXfRBuPdq7Icu13by0Nn51fU61K2VAR3a6Pie1BHeMZRmRGjyh4r5oG+eHCv6Ga2X52wkZnb6iFUuvMn2Ugvvnw=="]},"evidence":{"evidence":[]},"last_commit":{"height":"1359532","round":0,"block_id":{"hash":"7E2F61B7151FEB1D75AB7B0AC6CA6B5CCAF4F2E1A357A0628E81685DD5D883B4","parts":{"total":1,"hash":"643E2069080D49C0E8FDB415D4A6841663FDB75614AB394CC5F48886A7ECE0FB"}},"signatures":[{"block_id_flag":2,"validator_address":"51C5DEC2A8C0D876D8D799A096F19563F2C1273B","timestamp":"2022-11-30T23:04:14.235350753Z","signature":"eYgn7Geq9Kc6hSzgZut/mOHQx0HHfj+oSvDwTIM8jJCVlAn+rqp6eod7olyVy+42hWUNxtq6HBr1/167k0n9Dg=="},{"block_id_flag":2,"validator_address":"ED7E171129F79AB4D770433AC4CD1E6D121B57D6","timestamp":"2022-11-30T23:04:14.239212645Z","signature":"cENBbaH1nHGhggld9OANPkVwMQpSGcukUDtgQt5J3jCTqB7fTFKd5QJD/FwC3yqNvQJpbljVJj3Ge3sfHn2cCQ=="},{"block_id_flag":2,"validator_address":"A3BE52665F43F5A3200A4BEF6C670C978226B36F","timestamp":"2022-11-30T23:04:14.238581694Z","signature":"Acgz9dx86s1XnHuBz+zMJF6g5oJ92PvE0pzl/d7tLdg3x8dfnYj5Zt/PJ1/NCD3Gd3fj6OONlZiaESkKhheLCA=="},{"block_id_flag":2,"validator_address":"4F58BE62FB31F82BCAE53127A8ED030E5FE870FE","timestamp":"2022-11-30T23:04:14.238844376Z","signature":"D4+SsLDkCWuBzeRlDuRpZxnsLJFTDbxkI1QrlscJZmon8eQPvoiecifqbPm5blD4LH4De8AgrANAINMZyQkpCg=="},{"block_id_flag":2,"validator_address":"243FEF5563CA54682C1A187A83BB2EE5F1F24EC9","timestamp":"2022-11-30T23:04:14.138341689Z","signature":"jlbLJAY/BH41yg9cEUgzvVLcQYJYorw8zRN8HOnfP9UVYkrmWTXMw0WgJQw8jvesp/Sq0xsDaWHuJrRoGD7RCg=="},{"block_id_flag":2,"validator_address":"F3C67A5C47642658D97FD6110CE8326062A467D7","timestamp":"2022-11-30T23:04:14.138077895Z","signature":"x99Ywgy2t2+NrLwWBcB3wevhckJ08V6wl6YAPNdVSvggxxt9+eoQfmNIRGECw4t02g9elf7kE2cdxO2eo/ctAg=="},{"block_id_flag":2,"validator_address":"3130821CF2DAA1C00BA599C4D05C51D54ACE2B9E","timestamp":"2022-11-30T23:04:14.286272091Z","signature":"gboXioq/TwGiUG4Bkg0XklGLbEUoGl+Dnf4UTjfsAZkxBaR+pRMj4GUFW23jBPis7vIpNoUriOqPEgSqkg8tDw=="}]}}}}`,
				// abci_info
				`{"jsonrpc":"2.0","id":1,"result":{"response":{"data":"Xpla","version":"v1.1.2-cube","last_block_height":"1359640","last_block_app_hash":"wCZpDOY0V6x0WXmcW+P7kUTD3DJpZatwEdRyrgDZaK0="}}}`,
			},
			txinfo.LegacyTxInfo{
				TxID:            "7a13cb946589d07834119e3d9f3bf27e38da9990894e24850323582a404de46b",
				From:            "xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg",
				To:              "xpla1a8f3wnn7qwvwdzxkc9w849kfzhrr6gdvy4c8wv",
				ToAlt:           "",
				ContractAddress: "XPLA",
				Amount:          xc.NewAmountBlockchainFromUint64(5000000000000000),
				Fee:             xc.NewAmountBlockchainFromUint64(112200000000000000),
				FeePayer:        "xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg",
				BlockIndex:      1359533,
				BlockTime:       1669849454,
				Confirmations:   107,
				Sources:         nil,
				Destinations:    nil,
				Error:           "{\"error\": \"no money\"}",
				Status:          xc.TxStatusFailure,
			},
			"",
		},
		{
			xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			"E9C24C2E23CDCA56C8CE87A583149F8F88E75923F0CD958C003A84F631948978",
			`{}`,
			txinfo.LegacyTxInfo{},
			"response ID (0) does not match request ID (1)",
		},
		{
			xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			"E9C24C2E23CDCA56C8CE87A583149F8F88E75923F0CD958C003A84F631948978",
			`null`,
			txinfo.LegacyTxInfo{},
			"response ID (0) does not match request ID (1)",
		},
		{
			xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			"E9C24C2E23CDCA56C8CE87A583149F8F88E75923F0CD958C003A84F631948978",
			fmt.Errorf(`{"message": "custom RPC error", "code": 123}`),
			txinfo.LegacyTxInfo{},
			"custom RPC error",
		},
		{
			xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			"",
			"",
			txinfo.LegacyTxInfo{},
			"error unmarshalling: invalid character",
		},
		{
			xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			"invalid-sig",
			"",
			txinfo.LegacyTxInfo{},
			"encoding/hex: invalid byte",
		},
		{
			// should be able to catch this as TransactionNotFound case
			xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			"E9C24C2E23CDCA56C8CE87A583149F8F88E75923F0CD958C003A84F631948978",
			fmt.Errorf(`{"message": "RPC error -32603 - Internal error: tx (E97DB7DB40A02F0773EFE3AA5328292EDB27BEB089DF0972A26E8683068BCFA7) not found"}`),
			txinfo.LegacyTxInfo{},
			"TransactionNotFound:",
		},
	}

	for i, v := range vectors {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			server, close := testtypes.MockJSONRPC(t, v.resp)
			defer close()

			asset := v.asset
			asset.GetChain().URL = server.URL
			v.asset.GetChain().Limiter = rate.NewLimiter(rate.Inf, 1)
			client, _ := client.NewClient(asset)
			txInfo, err := client.FetchLegacyTxInfo(context.Background(), xc.TxHash(v.tx))

			if v.err != "" {
				require.ErrorContains(t, err, v.err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, txInfo)
				require.Equal(t, v.val, txInfo)
			}
		})
	}
}

func TestFetchBalance(t *testing.T) {

	vectors := []struct {
		asset    xc.ITask
		contract xc.ContractAddress
		address  string
		resp     interface{}
		val      string
		err      string
	}{
		{
			// Terra
			asset:   xc.NewChainConfig(xc.LUNA).WithChainCoin("uluna").WithChainPrefix("terra"),
			address: "terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
			resp:    `{"response": {"code": 0,"log": "","info": "","index": "0","key": null,"value": "ChAKBXVsdW5hEgc0OTc5MDYz","proofOps": null,"height": "2803726","codespace": ""}}`,
			val:     "4979063",
			err:     "",
		},
		{
			// XPLA
			asset:   xc.NewChainConfig(xc.XPLA).WithChainCoin("axpla").WithChainPrefix("xpla"),
			address: "xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg",
			resp:    `{"response": {"code": 0,"log": "","info": "","index": "0","key": null,"value": "Ch0KBWF4cGxhEhQ5OTY0ODQwMDAwMDAwMDAwMDAwMA==","proofOps": null,"height": "1329788","codespace": ""}}`,
			val:     "99648400000000000000",
			err:     "",
		},
		{
			// Injective peggy asset
			// asset: &xc.TokenAssetConfig{Asset: "USDT", Contract: "peggy0x3506424F91fD33084466F402d5D97f05F8e3b4AF", Decimals: 6,
			// },
			asset:    xc.NewChainConfig(xc.INJ).WithChainCoin("uinj").WithChainPrefix("inj"),
			contract: "peggy0x3506424F91fD33084466F402d5D97f05F8e3b4AF",
			address:  "inj162x3ax7z6ksquhshlqh6d498kr60qdx7wqf9we",
			resp: `{
				"jsonrpc": "2.0",
				"id": 0,
				"result": {
				  "response": {
					"code": 0,
					"log": "",
					"info": "",
					"index": "0",
					"key": null,
					"value": "Ch4KA2luahIXMzc0NTY3NDI4OTk5MjUwMDAwMDAwMDA=",
					"proofOps": null,
					"height": "11528367",
					"codespace": ""
				  }
				}
			  }`,
			val: "37456742899925000000000",
			err: "",
		},
		{
			// Terra cw20 asset
			// asset: &xc.TokenAssetConfig{Asset: "USDC", Contract: "terra1pepwcav40nvj3kh60qqgrk8k07ydmc00xyat06", Decimals: 6,
			asset: xc.NewChainConfig(xc.LUNC).WithChainCoin("uluna").WithChainPrefix("terra"),
			// },
			contract: "terra1pepwcav40nvj3kh60qqgrk8k07ydmc00xyat06",
			address:  "terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
			resp: []string{
				// first response fails because not a bank asset.
				`{"jsonrpc":"2.0","id":0,"result":{"response":{"code":1,"log":"denom does not exist","info":"","index":"0","key":null,"value":"","proofOps":null,"height":"12817698","codespace":""}}}`,
				`{"jsonrpc":"2.0","id":1,"result":{"response":{"code":0,"log":"","info":"","index":"0","key":null,"value":"ChZ7ImJhbGFuY2UiOiI0Mzk4NDEyNyJ9","proofOps":null,"height":"12817698","codespace":""}}}`,
			},
			val: "43984127",
			err: "",
		},
		{
			asset:   xc.NewChainConfig(xc.XPLA).WithChainCoin("axpla").WithChainPrefix("xpla"),
			address: "xpla-invalid",
			resp:    `null`,
			val:     "0",
			err:     "bad address",
		},
		{
			asset:   xc.NewChainConfig(xc.XPLA).WithChainCoin("axpla").WithChainPrefix("xpla"),
			address: "xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg",
			resp:    `null`,
			val:     "0",
			err:     "failed to get account balance",
		},
		{
			asset:   xc.NewChainConfig(xc.XPLA).WithChainCoin("axpla").WithChainPrefix("xpla"),
			address: "xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg",
			resp:    `{}`,
			val:     "0",
			err:     "failed to get account balance",
		},
		{
			asset:   xc.NewChainConfig(xc.XPLA).WithChainCoin("axpla").WithChainPrefix("xpla"),
			address: "xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg",
			resp:    fmt.Errorf(`{"message": "custom RPC error", "code": 123}`),
			val:     "",
			err:     "custom RPC error",
		},
	}

	for i, v := range vectors {
		fmt.Println("==testcase", i)
		server, close := testtypes.MockJSONRPC(t, v.resp)
		defer close()

		asset := v.asset
		asset.GetChain().URL = server.URL
		v.asset.GetChain().Limiter = rate.NewLimiter(rate.Inf, 1)

		args := xclient.NewBalanceArgs(xc.Address(v.address))
		if v.contract != "" {
			args = xclient.NewBalanceArgs(
				xc.Address(v.address),
				xclient.BalanceOptionContract(xc.ContractAddress(v.contract)),
			)
		}
		client, _ := client.NewClient(asset)

		balance, err := client.FetchBalance(context.Background(), args)

		if v.err != "" {
			require.Equal(t, "0", balance.String())
			require.ErrorContains(t, err, v.err)
		} else {
			require.NoError(t, err)
			require.NotNil(t, balance)
			require.Equal(t, v.val, balance.String())
		}
	}
}
