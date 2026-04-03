package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/xlm/common"
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
}

type sorobanSimulateResponse struct {
	Jsonrpc string                 `json:"jsonrpc"`
	Id      int                    `json:"id"`
	Result  *sorobanSimulateResult `json:"result,omitempty"`
	Error   *sorobanRpcError       `json:"error,omitempty"`
}

type sorobanSimulateResult struct {
	TransactionData string      `json:"transactionData"`
	MinResourceFee  json.Number `json:"minResourceFee"`
	Error           string      `json:"error,omitempty"`
}

type sorobanRpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// estimateSorobanResourceFee builds a temporary InvokeHostFunction transaction
// with natively constructed auth and footprint, then simulates it via Soroban RPC
// to get an accurate resource fee estimate. Only the fee is used from the response.
func (client *Client) estimateSorobanResourceFee(sorobanUrl string, args xcbuilder.TransferArgs, txInput *xlminput.TxInput) error {
	from := args.GetFrom()
	to := args.GetTo()

	contract, ok := args.GetContract()
	if !ok {
		return fmt.Errorf("contract is required for Soroban SAC transfers")
	}
	contractDetails, err := common.GetAssetAndIssuerFromContract(string(contract))
	if err != nil {
		return fmt.Errorf("failed to parse contract: %w", err)
	}
	xdrAsset, err := common.CreateAssetFromContractDetails(contractDetails)
	if err != nil {
		return fmt.Errorf("failed to create asset: %w", err)
	}
	sacId, err := xdrAsset.ContractID(txInput.Passphrase)
	if err != nil {
		return fmt.Errorf("failed to derive SAC contract ID: %w", err)
	}
	contractId := xdr.ContractId(sacId)
	contractAddr := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &contractId,
	}

	fromScVal, err := common.ScValAddress(string(from))
	if err != nil {
		return fmt.Errorf("failed to encode from address: %w", err)
	}
	toScVal, err := common.ScValAddress(string(to))
	if err != nil {
		return fmt.Errorf("failed to encode to address: %w", err)
	}
	amountScVal := common.ScValI128(args.GetAmount().Int().Int64())

	invokeArgs := xdr.InvokeContractArgs{
		ContractAddress: contractAddr,
		FunctionName:    "transfer",
		Args:            []xdr.ScVal{fromScVal, toScVal, amountScVal},
	}

	hostFn, err := xdr.NewHostFunction(xdr.HostFunctionTypeHostFunctionTypeInvokeContract, invokeArgs)
	if err != nil {
		return fmt.Errorf("failed to create host function: %w", err)
	}

	opBody, err := xdr.NewOperationBody(xdr.OperationTypeInvokeHostFunction, xdr.InvokeHostFunctionOp{
		HostFunction: hostFn,
	})
	if err != nil {
		return fmt.Errorf("failed to create operation body: %w", err)
	}

	txSourceAccount, err := common.MuxedAccountFromAddress(from)
	if err != nil {
		return fmt.Errorf("invalid source address: %w", err)
	}

	simTxe := xdr.TransactionV1Envelope{
		Tx: xdr.Transaction{
			SourceAccount: txSourceAccount,
			Fee:           xdr.Uint32(txInput.MaxFee),
			SeqNum:        xdr.SequenceNumber(txInput.GetXlmSequence()),
			Cond:          xdr.Preconditions{Type: xdr.PreconditionTypePrecondNone},
			Operations: []xdr.Operation{
				{
					SourceAccount: &txSourceAccount,
					Body:          opBody,
				},
			},
		},
	}

	simEnvelope, err := xdr.NewTransactionEnvelope(xdr.EnvelopeTypeEnvelopeTypeTx, simTxe)
	if err != nil {
		return fmt.Errorf("failed to create simulation envelope: %w", err)
	}

	envBytes, err := simEnvelope.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal simulation envelope: %w", err)
	}

	// Call Soroban RPC simulateTransaction
	reqBody := sorobanRpcRequest{
		Jsonrpc: "2.0",
		Id:      1,
		Method:  "simulateTransaction",
		Params: sorobanSimulateParams{
			Transaction: base64.StdEncoding.EncodeToString(envBytes),
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal simulate request: %w", err)
	}

	resp, err := client.HttpClient.Post(sorobanUrl, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to call soroban simulateTransaction: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read simulate response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("soroban simulateTransaction returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var simResp sorobanSimulateResponse
	if err := json.Unmarshal(respBytes, &simResp); err != nil {
		return fmt.Errorf("failed to unmarshal simulate response: %w", err)
	}

	if simResp.Error != nil {
		return fmt.Errorf("soroban rpc error (code %d): %s", simResp.Error.Code, simResp.Error.Message)
	}

	if simResp.Result == nil {
		return fmt.Errorf("soroban simulation returned no result")
	}

	if simResp.Result.Error != "" {
		return fmt.Errorf("soroban simulation error: %s", simResp.Result.Error)
	}

	// Use the resource fee from simulation
	resourceFee, err := simResp.Result.MinResourceFee.Int64()
	if err != nil {
		return fmt.Errorf("failed to parse resource fee: %w", err)
	}
	txInput.SorobanResourceFee = uint32(resourceFee)

	// Parse resource limits from the simulation's transactionData
	if simResp.Result.TransactionData != "" {
		dataBz, err := base64.StdEncoding.DecodeString(simResp.Result.TransactionData)
		if err == nil {
			var simData xdr.SorobanTransactionData
			if err := simData.UnmarshalBinary(dataBz); err == nil {
				txInput.SorobanInstructions = uint32(simData.Resources.Instructions)
				txInput.SorobanDiskReadBytes = uint32(simData.Resources.DiskReadBytes)
				txInput.SorobanWriteBytes = uint32(simData.Resources.WriteBytes)
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
