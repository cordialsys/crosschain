package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/client/types"
	"github.com/sirupsen/logrus"
)

type XrpError struct {
	Result struct {
		ErrorStatus  string `json:"error"`
		ErrorMessage string `json:"error_message"`
		ErrorCode    int    `json:"error_code"`
	} `json:"result"`
}

func (err *XrpError) Error() string {
	return fmt.Sprintf("%s: %s (code: %d)", err.Result.ErrorStatus, err.Result.ErrorMessage, err.Result.ErrorCode)
}

func (client *Client) Send(method string, requestBody any, response any) error {

	jsonPayload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request payload: %w", err)
	}

	request, err := http.NewRequest(method, client.Url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	logrus.WithField("method", method).WithField("params", string(jsonPayload)).Debug("request")

	resp, err := client.HttpClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch balance, HTTP status: %s", resp.Status)
	}
	bz, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var errMaybe XrpError
	_ = json.Unmarshal(bz, &errMaybe)
	if errMaybe.Result.ErrorStatus != "" || errMaybe.Result.ErrorMessage != "" {
		return &errMaybe
	}

	logrus.WithField("body", string(bz)).Debug("response")
	err = json.Unmarshal(bz, response)
	if err != nil {
		return fmt.Errorf("failed to decode response body: %w", err)
	}

	return nil
}

func (client *Client) getAccountInfo(address xc.Address) (*types.AccountInfoResponse, error) {
	request := types.AccountInfoRequest{
		Method: "account_info",
		Params: []types.AccountInfoParamEntry{
			{
				Account:     address,
				LedgerIndex: types.Validated,
			},
		},
	}

	var accountInfoResponse types.AccountInfoResponse
	err := client.Send(MethodPost, request, &accountInfoResponse)
	if err != nil {
		return nil, err
	}

	return &accountInfoResponse, nil
}

func (client *Client) getLedger(index types.LedgerIndex, transactions bool) (*types.LedgerResponse, error) {
	ledgerRequest := types.LedgerRequest{
		Method: "ledger",
		Params: []types.LedgerParamEntry{
			{
				LedgerIndex:  index,
				Transactions: transactions,
			},
		},
	}

	var ledgerResponse types.LedgerResponse
	err := client.Send(MethodPost, ledgerRequest, &ledgerResponse)
	if err != nil {
		return nil, err
	}

	return &ledgerResponse, nil
}

func (client *Client) getLedgerData(index types.LedgerIndex) (*types.LedgerDataResponse, error) {
	ledgerRequest := types.LedgerDataRequest{
		Method: "ledger_data",
		Params: []types.LedgerDataParams{
			{
				LedgerIndex: index,
				Limit:       1,
			},
		},
	}

	var ledgerResponse types.LedgerDataResponse
	err := client.Send(MethodPost, ledgerRequest, &ledgerResponse)
	if err != nil {
		return nil, err
	}

	return &ledgerResponse, nil
}

func (client *Client) getLatestLedger(transactions bool) (*types.LedgerResponse, error) {
	return client.getLedger(types.Current, transactions)
}

func (client *Client) getAccountLines(address xc.Address) (*types.AccountLinesResponse, error) {
	request := types.AccountLinesRequest{
		Method: "account_lines",
		Params: []types.AccountLinesParamEntry{
			{
				Account: address,
			},
		},
	}

	var accountLinesResponse types.AccountLinesResponse
	err := client.Send(MethodPost, request, &accountLinesResponse)
	if err != nil {
		return nil, err
	}
	return &accountLinesResponse, nil
}

func (client *Client) postSubmit(serializedTxInputHexBytes []byte) (*types.SubmitResponse, error) {
	submitRequest := &types.SubmitRequest{
		Method: "submit",
		Params: []types.SubmitParamEntry{
			{
				TxBlob: string(serializedTxInputHexBytes),
			},
		},
	}

	var submitResponse types.SubmitResponse
	err := client.Send(MethodPost, submitRequest, &submitResponse)
	if err != nil {
		return nil, err
	}

	return &submitResponse, nil
}

func (client *Client) getFee() (*types.FeeResponse, error) {
	submitRequest := &types.FeeRequest{
		Method: "fee",
		Params: []types.FeeParams{
			types.FeeParams{},
		},
	}

	var submitResponse types.FeeResponse
	err := client.Send(MethodPost, submitRequest, &submitResponse)
	if err != nil {
		return nil, err
	}

	return &submitResponse, nil
}
