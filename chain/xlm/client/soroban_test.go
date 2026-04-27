package client

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/xlm/common"
	txinput "github.com/cordialsys/crosschain/chain/xlm/tx_input"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/require"
)

func TestApplySorobanSimulationDataStoresTransactionData(t *testing.T) {
	simData := xdr.SorobanTransactionData{
		Resources: xdr.SorobanResources{
			Instructions:  123,
			DiskReadBytes: 456,
			WriteBytes:    789,
		},
		ResourceFee: 111,
	}
	simDataBytes, err := simData.MarshalBinary()
	require.NoError(t, err)

	input := &txinput.TxInput{}
	encoded := base64.StdEncoding.EncodeToString(simDataBytes)
	require.NoError(t, applySorobanSimulationData(input, encoded))

	require.Equal(t, uint32(123), input.SorobanInstructions)
	require.Equal(t, uint32(456), input.SorobanDiskReadBytes)
	require.Equal(t, uint32(789), input.SorobanWriteBytes)
	require.Equal(t, encoded, input.SorobanTransactionData)
}

func TestApplySorobanSimulationAuthStoresSubInvocationEntries(t *testing.T) {
	input := &txinput.TxInput{}
	authEntry := mustAuthEntryWithSubInvocations(t)
	require.NoError(t, applySorobanSimulationAuth(input, []sorobanSimulateHostFunctionResult{
		{Auth: []string{authEntry}},
	}))

	require.Equal(t, []string{authEntry}, input.SorobanAuthorizationEntries)
}

func TestEstimateSorobanResourceFeeSupportsNativeXLM(t *testing.T) {
	simData := xdr.SorobanTransactionData{
		Resources: xdr.SorobanResources{
			Instructions:  123,
			DiskReadBytes: 456,
			WriteBytes:    789,
		},
		ResourceFee: 111,
	}
	simDataBytes, err := simData.MarshalBinary()
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sorobanRpcRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req.Method {
		case "simulateTransaction":
			params, ok := req.Params.(map[string]interface{})
			require.True(t, ok)
			txBase64, ok := params["transaction"].(string)
			require.True(t, ok)
			txBytes, err := base64.StdEncoding.DecodeString(txBase64)
			require.NoError(t, err)
			var envelope xdr.TransactionEnvelope
			require.NoError(t, envelope.UnmarshalBinary(txBytes))
			require.Equal(t, xdr.OperationTypeInvokeHostFunction, envelope.Operations()[0].Body.Type)
			_, ok = envelope.V1.Tx.Ext.GetSorobanData()
			require.True(t, ok)

			require.NoError(t, json.NewEncoder(w).Encode(sorobanSimulateResponse{
				Jsonrpc: "2.0",
				Id:      1,
				Result: &sorobanSimulateResult{
					TransactionData: base64.StdEncoding.EncodeToString(simDataBytes),
					MinResourceFee:  json.Number("222"),
					Results: []sorobanSimulateHostFunctionResult{
						{Auth: []string{mustAuthEntryWithSubInvocations(t)}},
					},
				},
			}))
		case "getFeeStats":
			require.NoError(t, json.NewEncoder(w).Encode(sorobanFeeStatsResponse{
				Jsonrpc: "2.0",
				Id:      1,
				Result: &struct {
					SorobanInclusionFee struct {
						P90  json.Number `json:"p90"`
						P80  json.Number `json:"p80"`
						P50  json.Number `json:"p50"`
						Mode json.Number `json:"mode"`
					} `json:"sorobanInclusionFee"`
				}{
					SorobanInclusionFee: struct {
						P90  json.Number `json:"p90"`
						P80  json.Number `json:"p80"`
						P50  json.Number `json:"p50"`
						Mode json.Number `json:"mode"`
					}{
						P90: json.Number("333"),
					},
				},
			}))
		default:
			t.Fatalf("unexpected method: %s", req.Method)
		}
	}))
	defer server.Close()

	chain := xc.NewChainConfig(xc.XLM)
	args, err := xcbuilder.NewTransferArgs(
		chain.Base(),
		xc.Address("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H"),
		xc.Address("CBP4GFAK4GDKCMVLNIHRZPEAPT7CFYTBKICOXCVMP2FEN3QFKCRZ27KS"),
		xc.NewAmountBlockchainFromUint64(10),
	)
	require.NoError(t, err)

	input := txinput.NewTxInput("Test SDF Network ; September 2015")
	input.MaxFee = 100
	client := &Client{HttpClient: server.Client(), Asset: chain}
	require.NoError(t, client.estimateSorobanResourceFee(server.URL, args, input))
	require.Equal(t, uint32(222), input.SorobanResourceFee)
	require.Equal(t, uint32(333), input.MaxFee)
	require.Equal(t, uint32(123), input.SorobanInstructions)
	require.Equal(t, uint32(456), input.SorobanDiskReadBytes)
	require.Equal(t, uint32(789), input.SorobanWriteBytes)
	require.NotEmpty(t, input.SorobanTransactionData)
	require.NotEmpty(t, input.SorobanAuthorizationEntries)
}

func mustAuthEntryWithSubInvocations(t *testing.T) string {
	t.Helper()
	contractAddr, err := common.ScAddressFromString("CBP4GFAK4GDKCMVLNIHRZPEAPT7CFYTBKICOXCVMP2FEN3QFKCRZ27KS")
	require.NoError(t, err)
	invokeArgs := xdr.InvokeContractArgs{
		ContractAddress: contractAddr,
		FunctionName:    "transfer",
	}
	contractFn, err := xdr.NewSorobanAuthorizedFunction(
		xdr.SorobanAuthorizedFunctionTypeSorobanAuthorizedFunctionTypeContractFn,
		invokeArgs,
	)
	require.NoError(t, err)
	entry := xdr.SorobanAuthorizationEntry{
		Credentials: xdr.SorobanCredentials{
			Type: xdr.SorobanCredentialsTypeSorobanCredentialsSourceAccount,
		},
		RootInvocation: xdr.SorobanAuthorizedInvocation{
			Function: contractFn,
			SubInvocations: []xdr.SorobanAuthorizedInvocation{
				{Function: contractFn},
			},
		},
	}
	entryBz, err := entry.MarshalBinary()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(entryBz)
}
