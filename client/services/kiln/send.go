package kiln

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/sirupsen/logrus"
)

type Client struct {
	Chain  string
	Url    string
	ApiKey string
}
type Error struct {
	Message string `json:"message"`
}

func ensure0x(val string) string {
	if !strings.HasPrefix(val, "0x") {
		return "0x" + val
	}
	return val
}

func NewClient(chain, url, apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api-key required")
	}

	return &Client{
		Chain:  chain,
		Url:    url,
		ApiKey: apiKey,
	}, nil
}

func (cli *Client) Get(path string, response any) error {
	return cli.Send("GET", path, nil, response)
}

func (cli *Client) GetAndPrint(path string) error {
	var res json.RawMessage
	err := cli.Send("GET", path, nil, &res)
	fmt.Println(string(res))
	return err
}

func (cli *Client) Post(path string, requestBody any, response any) error {
	return cli.Send("POST", path, requestBody, response)
}
func (cli *Client) Send(method string, path string, requestBody any, response any) error {
	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("%s/%s", cli.Url, path)
	var request *http.Request
	var err error
	if requestBody == nil {
		request, err = http.NewRequest(method, url, nil)
	} else {
		bz, _ := json.Marshal(requestBody)
		request, err = http.NewRequest(method, url, bytes.NewBuffer(bz))
		if err == nil {
			request.Header.Add("content-type", "application/json")
		}
	}
	if err != nil {
		return err
	}
	if cli.ApiKey != "" {
		request.Header.Add("authorization", "Bearer "+cli.ApiKey)
	}
	logrus.WithField("url", url).Debug(method)
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to GET: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}
	logrus.WithFields(logrus.Fields{
		"body":   string(body),
		"status": resp.StatusCode,
	}).Debug("response")

	if resp.StatusCode == http.StatusOK || resp.StatusCode == 201 {
		if response != nil {
			if err := json.Unmarshal(body, response); err != nil {
				return fmt.Errorf("failed to unmarshal response: %v", err)
			}
		}
		return nil
	} else {
		// Deserialize to ErrorResponse struct for other status codes
		var errorResponse Error
		logrus.WithField("body", string(body)).Debug("error")
		if err := json.Unmarshal(body, &errorResponse); err != nil {
			return fmt.Errorf("failed to unmarshal error response: %v", err)
		}
		if errorResponse.Message != "" {
			return fmt.Errorf("%s", errorResponse.Message)
		}
		logrus.WithField("body", string(body)).WithField("chain", cli.Chain).Warn("unknown kiln error")
		return fmt.Errorf("unknown kiln error (%d)", resp.StatusCode)
	}
}

func (cli *Client) ResolveAccount(accountIdMaybe string) (*Account, error) {
	var acc GetAccountResponse
	if accountIdMaybe != "" {
		err := cli.Get("v1/accounts/"+accountIdMaybe, &acc)
		if err != nil {
			return &acc.Data, fmt.Errorf("could not locate account: %v", err)
		}
		return &acc.Data, nil
	}
	var accRes GetAccountsResponse
	err := cli.Get("v1/accounts", &accRes)
	if err != nil {
		return nil, err
	}
	if len(accRes.Data) == 0 {
		return nil, fmt.Errorf("no accounts in kiln organization, must create one")
	}
	if len(accRes.Data) != 1 {
		return nil, fmt.Errorf("multiple accounts in kiln organization, must provide account ID to use")
	}
	return &accRes.Data[0], nil
}

func (cli *Client) CreateValidatorKeys(accountId string, address string, count int) (*CreateValidatorKeysResponse, error) {
	// not clear which response format kiln is switching to, so we try both.
	var res1 CreateValidatorKeysResponse1
	var res2 CreateValidatorKeysResponse2
	var resRaw json.RawMessage
	err := cli.Post("v1/eth/keys", &CreateValidatorKeysRequest{
		// use default
		Format:            BatchDeposit,
		AccountID:         accountId,
		WithdrawalAddress: address,
		// Use default same as withdraw address
		FeeRecipientAddress: address,
		Number:              count,
	}, &resRaw)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(resRaw, &res1)
	if err != nil {
		err = json.Unmarshal(resRaw, &res2)
		if err != nil {

			return nil, err
		}
		return &CreateValidatorKeysResponse{
			Response2: &res2,
		}, nil
	}

	return &CreateValidatorKeysResponse{
		Response1: &res1,
	}, nil
}

func (cli *Client) GenerateStakeTransaction(accountId string, address string, amount xc.AmountBlockchain) (*GenerateTransactionResponse, error) {
	var res GenerateTransactionResponse
	err := cli.Post("v1/eth/transaction/stake", &GenerateTransactionRequest{
		AccountID: accountId,
		Wallet:    address,
		AmountWei: amount.String(),
	}, &res)
	return &res, err
}

func (cli *Client) GetStakesByValidator(validator string) (*GetStakesResponse, error) {
	var res GetStakesResponse
	err := cli.Get(fmt.Sprintf("v1/eth/stakes?validators=%s", ensure0x(validator)), &res)
	return &res, err
}

func (cli *Client) GetStakesByOwner(address string) (*GetStakesResponse, error) {
	var res GetStakesResponse
	err := cli.Get(fmt.Sprintf("v1/eth/stakes?wallets=%s", ensure0x(address)), &res)
	return &res, err
}

func (cli *Client) GetAllStakesByOwner(address string) ([]StakeAccount, error) {
	stakes := []StakeAccount{}
	pageSize := 50
	currentPage := 0
	totalPages := 1
	for currentPage < totalPages {
		var res GetStakesResponse
		err := cli.Get(fmt.Sprintf("v1/eth/stakes?wallets=%s&current_page=%d&page_size=%d", ensure0x(address), (currentPage+1), pageSize), &res)
		if err != nil {
			return nil, err
		}
		if len(res.Data) == 0 {
			break
		}
		stakes = append(stakes, res.Data...)
		currentPage += 1
		totalPages = res.Pagination.TotalPages
	}
	return stakes, nil
}

func (cli *Client) OperationsByOwner(address string) (*OperationsResponse, error) {
	var res OperationsResponse
	err := cli.Get(fmt.Sprintf("v1/eth/operations?wallets=%s", ensure0x(address)), &res)
	return &res, err
}
