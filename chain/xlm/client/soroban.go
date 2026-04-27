package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	xcbuilder "github.com/cordialsys/crosschain/builder"
	xlmbuilder "github.com/cordialsys/crosschain/chain/xlm/builder"
	xlmtx "github.com/cordialsys/crosschain/chain/xlm/tx"
	xlminput "github.com/cordialsys/crosschain/chain/xlm/tx_input"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type sorobanRpcRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	Id      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type sorobanSimulateParams struct {
	Transaction string `json:"transaction"`
	AuthMode    string `json:"authMode,omitempty"`
}

type sorobanSimulateResponse struct {
	Jsonrpc string                 `json:"jsonrpc"`
	Id      int                    `json:"id"`
	Result  *sorobanSimulateResult `json:"result,omitempty"`
	Error   *sorobanRpcError       `json:"error,omitempty"`
}

type sorobanSimulateResult struct {
	TransactionData string                              `json:"transactionData"`
	MinResourceFee  json.Number                         `json:"minResourceFee"`
	Error           string                              `json:"error,omitempty"`
	Results         []sorobanSimulateHostFunctionResult `json:"results,omitempty"`
}

type sorobanSimulateHostFunctionResult struct {
	Auth []string `json:"auth,omitempty"`
}

type sorobanRpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// estimateSorobanResourceFee builds the transaction through the XLM TxBuilder,
// then simulates that unsigned envelope via Soroban RPC to get resource
// estimates, transaction data, and any recorded auth sub-invocations.
func (client *Client) estimateSorobanResourceFee(sorobanUrl string, args xcbuilder.TransferArgs, txInput *xlminput.TxInput) error {
	simulationTx, err := client.buildSorobanSimulationTransaction(args, txInput)
	if err != nil {
		return err
	}

	simResult, err := client.simulateSorobanTransaction(sorobanUrl, simulationTx, "")
	if err != nil {
		return err
	}

	// Use the resource fee from simulation
	resourceFee, err := simResult.MinResourceFee.Int64()
	if err != nil {
		return fmt.Errorf("failed to parse resource fee: %w", err)
	}
	txInput.SorobanResourceFee = uint32(resourceFee)

	if simResult.TransactionData != "" {
		if err := applySorobanSimulationData(txInput, simResult.TransactionData); err != nil {
			return err
		}
	}

	if authSimulationTx, err := buildSorobanAuthRecordingTransaction(simulationTx); err == nil {
		if authResult, err := client.simulateSorobanTransaction(sorobanUrl, authSimulationTx, "record"); err == nil {
			if err := applySorobanSimulationAuth(txInput, authResult.Results); err != nil {
				return err
			}
		}
	}

	// Fetch fee stats for an accurate inclusion fee
	inclusionFee, err := client.fetchSorobanInclusionFee(sorobanUrl)
	if err == nil && inclusionFee > 0 {
		txInput.MaxFee = inclusionFee
	}

	return nil
}

func (client *Client) simulateSorobanTransaction(sorobanUrl string, transaction string, authMode string) (*sorobanSimulateResult, error) {
	reqBody := sorobanRpcRequest{
		Jsonrpc: "2.0",
		Id:      1,
		Method:  "simulateTransaction",
		Params: sorobanSimulateParams{
			Transaction: transaction,
			AuthMode:    authMode,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal simulate request: %w", err)
	}

	resp, err := client.HttpClient.Post(sorobanUrl, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to call soroban simulateTransaction: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read simulate response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("soroban simulateTransaction returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var simResp sorobanSimulateResponse
	if err := json.Unmarshal(respBytes, &simResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal simulate response: %w", err)
	}

	if simResp.Error != nil {
		return nil, fmt.Errorf("soroban rpc error (code %d): %s", simResp.Error.Code, simResp.Error.Message)
	}

	if simResp.Result == nil {
		return nil, fmt.Errorf("soroban simulation returned no result")
	}

	if simResp.Result.Error != "" {
		return nil, fmt.Errorf("soroban simulation error: %s", simResp.Result.Error)
	}

	return simResp.Result, nil
}

func (client *Client) buildSorobanSimulationTransaction(args xcbuilder.TransferArgs, txInput *xlminput.TxInput) (string, error) {
	if client.Asset == nil {
		return "", fmt.Errorf("stellar client is missing asset configuration")
	}

	txBuilder, err := xlmbuilder.NewTxBuilder(client.Asset.Base())
	if err != nil {
		return "", fmt.Errorf("failed to create xlm tx builder: %w", err)
	}

	simulationInput := *txInput
	simulationInput.SorobanTransactionData = ""
	simulationInput.SorobanAuthorizationEntries = nil
	unsignedTx, err := txBuilder.Transfer(args, &simulationInput)
	if err != nil {
		return "", fmt.Errorf("failed to build soroban simulation transaction: %w", err)
	}

	xlmTx, ok := unsignedTx.(*xlmtx.Tx)
	if !ok {
		return "", fmt.Errorf("unexpected xlm transaction type %T", unsignedTx)
	}
	if xlmTx.TxEnvelope == nil {
		return "", fmt.Errorf("missing xlm transaction envelope")
	}

	envBytes, err := xlmTx.TxEnvelope.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to marshal simulation envelope: %w", err)
	}
	return base64.StdEncoding.EncodeToString(envBytes), nil
}

func buildSorobanAuthRecordingTransaction(transaction string) (string, error) {
	envBytes, err := base64.StdEncoding.DecodeString(transaction)
	if err != nil {
		return "", fmt.Errorf("failed to decode simulation envelope: %w", err)
	}

	var envelope xdr.TransactionEnvelope
	if err := envelope.UnmarshalBinary(envBytes); err != nil {
		return "", fmt.Errorf("failed to unmarshal simulation envelope: %w", err)
	}
	if envelope.Type != xdr.EnvelopeTypeEnvelopeTypeTx || envelope.V1 == nil {
		return "", fmt.Errorf("expected xlm transaction envelope")
	}

	envelope.V1.Tx.Ext = xdr.TransactionExt{}
	for i := range envelope.V1.Tx.Operations {
		invokeOp, ok := envelope.V1.Tx.Operations[i].Body.GetInvokeHostFunctionOp()
		if !ok {
			continue
		}
		invokeOp.Auth = nil
		body, err := xdr.NewOperationBody(xdr.OperationTypeInvokeHostFunction, invokeOp)
		if err != nil {
			return "", fmt.Errorf("failed to clear simulation auth: %w", err)
		}
		envelope.V1.Tx.Operations[i].Body = body
	}

	envBytes, err = envelope.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth recording envelope: %w", err)
	}
	return base64.StdEncoding.EncodeToString(envBytes), nil
}

func applySorobanSimulationData(txInput *xlminput.TxInput, transactionData string) error {
	dataBz, err := base64.StdEncoding.DecodeString(transactionData)
	if err != nil {
		return fmt.Errorf("failed to decode soroban transaction data: %w", err)
	}

	var simData xdr.SorobanTransactionData
	if err := simData.UnmarshalBinary(dataBz); err != nil {
		return fmt.Errorf("failed to unmarshal soroban transaction data: %w", err)
	}

	txInput.SorobanInstructions = uint32(simData.Resources.Instructions)
	txInput.SorobanDiskReadBytes = uint32(simData.Resources.DiskReadBytes)
	txInput.SorobanWriteBytes = uint32(simData.Resources.WriteBytes)
	txInput.SorobanTransactionData = transactionData

	return nil
}

func applySorobanSimulationAuth(txInput *xlminput.TxInput, results []sorobanSimulateHostFunctionResult) error {
	txInput.SorobanAuthorizationEntries = nil
	for _, result := range results {
		for _, authEntry := range result.Auth {
			entry, err := sorobanAuthorizationEntryFromBase64(authEntry)
			if err != nil {
				return err
			}
			if len(entry.RootInvocation.SubInvocations) == 0 {
				continue
			}
			txInput.SorobanAuthorizationEntries = append(txInput.SorobanAuthorizationEntries, authEntry)
		}
	}
	return nil
}

func sorobanAuthorizationEntryFromBase64(encoded string) (*xdr.SorobanAuthorizationEntry, error) {
	dataBz, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode soroban authorization entry: %w", err)
	}

	var entry xdr.SorobanAuthorizationEntry
	if err := entry.UnmarshalBinary(dataBz); err != nil {
		return nil, fmt.Errorf("failed to unmarshal soroban authorization entry: %w", err)
	}
	return &entry, nil
}

type sorobanFeeStatsResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Id      int    `json:"id"`
	Result  *struct {
		SorobanInclusionFee struct {
			P90  json.Number `json:"p90"`
			P80  json.Number `json:"p80"`
			P50  json.Number `json:"p50"`
			Mode json.Number `json:"mode"`
		} `json:"sorobanInclusionFee"`
	} `json:"result,omitempty"`
}

// fetchSorobanInclusionFee gets the current network inclusion fee from Soroban RPC.
// Prefers p90, falls back to p80, p50, then mode.
func (client *Client) fetchSorobanInclusionFee(sorobanUrl string) (uint32, error) {
	reqBody := sorobanRpcRequest{
		Jsonrpc: "2.0",
		Id:      1,
		Method:  "getFeeStats",
		Params:  nil,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return 0, err
	}

	resp, err := client.HttpClient.Post(sorobanUrl, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var feeResp sorobanFeeStatsResponse
	if err := json.Unmarshal(respBytes, &feeResp); err != nil {
		return 0, err
	}

	if feeResp.Result == nil {
		return 0, fmt.Errorf("no fee stats result")
	}

	fees := feeResp.Result.SorobanInclusionFee
	for _, candidate := range []json.Number{fees.P90, fees.P80, fees.P50, fees.Mode} {
		if v, err := candidate.Int64(); err == nil && v > 0 {
			return uint32(v), nil
		}
	}

	return 0, fmt.Errorf("no valid inclusion fee in fee stats")
}
