package httpclient

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"

	"github.com/okx/go-wallet-sdk/crypto/base58"
)

// Implement basic tron client that use's TRON's http api.
// This API is exposed on many public endpoints and is supported by private RPC providers.

// Bytes marshals/unmarshals as a JSON string with NO 0x prefix.
type Bytes []byte

var _ json.Unmarshaler = &Bytes{}

func (b *Bytes) UnmarshalJSON(inputBz []byte) error {
	var err error
	input := string(inputBz)
	input = strings.TrimPrefix(input, "\"")
	input = strings.TrimSuffix(input, "\"")
	input = strings.TrimPrefix(input, "0x")
	*b, err = hex.DecodeString(string(input))
	return err
}

type Client struct {
	baseUrl *url.URL
	client  *http.Client
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Error   string `json:"Error"`
}
type ContractParameter struct {
	Value   map[string]interface{} `json:"value"`
	TypeUrl string                 `json:"type_url"`
}
type ContractData struct {
	Parameter ContractParameter `json:"parameter"`
	Type      string            `json:"type"`
}
type Receipt struct {
	NetFee uint64 `json:"net_fee"`
}
type TransactionRawData struct {
	Contract          []ContractData `json:"contract"`
	RefBlockBytes     Bytes          `json:"ref_block_bytes"`
	RefBlockHashBytes Bytes          `json:"ref_block_hash"`
	Expiration        uint64         `json:"expiration"`
	FeeLimit          uint64         `json:"fee_limit"`
	Timestamp         uint64         `json:"timestamp"`
}
type CreateTransactionResponse struct {
	Error
	RawData    TransactionRawData `json:"raw_data"`
	RawDataHex Bytes              `json:"raw_data_hex"`
}
type GetTransactionIDResponse struct {
	Error
	RawData    TransactionRawData `json:"raw_data"`
	RawDataHex Bytes              `json:"raw_data_hex"`
	TxID       Bytes              `json:"txID"`
	Signature  []Bytes            `json:"signature"`
}

type GetTransactionInfoById struct {
	Error
	Id              Bytes    `json:"id"`
	Fee             uint64   `json:"fee"`
	BlockNumber     uint64   `json:"blockNumber"`
	BlockTimeStamp  uint64   `json:"blockTimeStamp"`
	ContractResult  []string `json:"contractResult"`
	Receipt         Receipt  `json:"receipt"`
	ContractAddress string   `json:"contract_address"`

	Logs                 []*Log                 `json:"log"`
	InternalTransactions []*InternalTransaction `json:"internal_transactions"`
}

type Log struct {
	Address Bytes   `json:"address"`
	Topics  []Bytes `json:"topics"`
	Data    Bytes   `json:"data"`
}
type InternalTransaction struct {
	Hash              Bytes `json:"hash"`
	CallerAddress     Bytes `json:"caller_address"`
	TransferToAddress Bytes `json:"transferTo_address"`
	Note              Bytes `json:"note"`
}
type BlockHeaderRawData struct {
	Number    uint64 `json:"number"`
	Verion    uint64 `json:"version"`
	Timestamp uint64 `json:"timestamp"`
	// other fields...
}

type BlockHeader struct {
	RawData          BlockHeaderRawData `json:"raw_data"`
	WitnessSignature Bytes              `json:"witness_signature"`
}
type BlockResponse struct {
	Error
	BlockHeader BlockHeader `json:"block_header"`
	BlockId     string      `json:"blockID"`
}

type TriggerConstantContractResponse struct {
	Error
	ConstantResult []Bytes `json:"constant_result"`
}

type GetAccountResponse struct {
	Error
	Balance uint64 `json:"balance"`
	Address string `json:"address"`
}

func NewHttpClient(baseUrl string) (*Client, error) {
	baseUrl = strings.TrimSuffix(baseUrl, "/")
	baseUrl = strings.TrimSuffix(baseUrl, "/wallet")
	baseUrl = strings.TrimSuffix(baseUrl, "/jsonrpc")
	u, err := url.Parse(baseUrl)

	// may want to pass externally to support additional
	// headers or something.
	client := &http.Client{}

	return &Client{
		baseUrl: u,
		client:  client,
	}, err
}

func parseResponse[T any](res *http.Response, dest T) (T, error) {
	bz, err := io.ReadAll(res.Body)
	if err != nil {
		return dest, err
	}
	err = json.Unmarshal(bz, dest)
	// decoder := json.NewDecoder(res.Body)
	// err := decoder.Decode(dest)
	return dest, err
}

func checkError(res Error) error {
	if len(res.Code) > 0 && len(res.Message) > 0 {
		return fmt.Errorf("%s: %s", res.Code, res.Message)
	}
	if len(res.Error) > 0 {
		return fmt.Errorf("%s", res.Error)
	}
	return nil
}

func postRequest(url string, body any) (*http.Request, error) {
	bz, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bz))
	if err != nil {
		return req, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (c *Client) Url(path string) string {
	return c.baseUrl.JoinPath(path).String()
}

func (c *Client) CreateTransaction(from string, to string, amount int) (*CreateTransactionResponse, error) {
	req, err := postRequest(c.Url("wallet/createtransaction"), map[string]interface{}{
		"owner_address": from,
		"to_address":    to,
		"amount":        amount,
		"visible":       true,
	})

	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	parsed, err := parseResponse(resp, &CreateTransactionResponse{})
	if err != nil {
		return nil, err
	}
	err = checkError(parsed.Error)
	if err != nil {
		return parsed, err
	}
	// if parsed.

	return parsed, nil
}

func (c *Client) BroadcastHex(txHex string) (*CreateTransactionResponse, error) {
	req, err := postRequest(c.Url("wallet/broadcasthex"), map[string]interface{}{
		"transaction": txHex,
	})

	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	parsed, err := parseResponse(resp, &CreateTransactionResponse{})
	if err != nil {
		return nil, err
	}
	err = checkError(parsed.Error)
	if err != nil {
		return parsed, err
	}

	return parsed, nil
}

func (c *Client) GetTransactionByID(txHash string) (*GetTransactionIDResponse, error) {
	req, err := postRequest(c.Url("wallet/gettransactionbyid"), map[string]interface{}{
		"value":   txHash,
		"visible": true,
	})

	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	parsed, err := parseResponse(resp, &GetTransactionIDResponse{})
	if err != nil {
		return nil, err
	}
	err = checkError(parsed.Error)
	if err != nil {
		return parsed, err
	}
	if len(parsed.TxID) == 0 {
		return parsed, fmt.Errorf("could not find tx: %s", txHash)
	}

	return parsed, nil
}

func (c *Client) GetTransactionInfoByID(txHash string) (*GetTransactionInfoById, error) {
	req, err := postRequest(c.Url("wallet/gettransactioninfobyid"), map[string]interface{}{
		"value": txHash,
	})

	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	parsed, err := parseResponse(resp, &GetTransactionInfoById{})
	if err != nil {
		return nil, err
	}
	err = checkError(parsed.Error)
	if err != nil {
		return parsed, err
	}
	if len(parsed.Id) == 0 {
		return parsed, fmt.Errorf("could not find tx info: %s", txHash)
	}

	return parsed, nil
}

func (c *Client) GetBlockByNum(num uint64) (*BlockResponse, error) {
	req, err := postRequest(c.Url("wallet/getblockbynum"), map[string]interface{}{
		"num": num,
	})

	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	parsed, err := parseResponse(resp, &BlockResponse{})
	if err != nil {
		return nil, err
	}
	err = checkError(parsed.Error)
	if err != nil {
		return parsed, err
	}
	if len(parsed.BlockId) == 0 {
		return parsed, fmt.Errorf("could not find block by num: %d", num)
	}

	return parsed, nil
}

func (c *Client) TriggerConstantContracts(ownerAddress string, contract string, funcSelector string, param string) (*TriggerConstantContractResponse, error) {
	req, err := postRequest(c.Url("wallet/triggerconstantcontract"), map[string]interface{}{
		"owner_address":     ownerAddress,
		"contract_address":  contract,
		"constant":          true,
		"function_selector": funcSelector,
		"parameter":         param,
		"visible":           true,
	})

	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	parsed, err := parseResponse(resp, &TriggerConstantContractResponse{})
	if err != nil {
		return nil, err
	}
	err = checkError(parsed.Error)
	if err != nil {
		return parsed, err
	}

	return parsed, nil
}

func (c *Client) ReadTrc20Balance(fromAddress string, contract string) (*big.Int, error) {
	addrB, _, err := base58.CheckDecode(fromAddress)
	if err != nil {
		return &big.Int{}, err
	}
	addrHex := hex.EncodeToString(addrB)
	contractB, _, err := base58.CheckDecode(contract)
	if err != nil {
		return &big.Int{}, err
	}
	req := "0000000000000000000000000000000000000000000000000000000000000000"[len(addrHex):] + addrHex
	ownerAddress := hex.EncodeToString(addrB)
	contractHex := hex.EncodeToString(contractB)
	_, _ = ownerAddress, contractHex

	response, err := c.TriggerConstantContracts(fromAddress, contract, "balanceOf(address)", req)
	if err != nil {
		return &big.Int{}, err
	}

	value := big.NewInt(0)
	if len(response.ConstantResult) == 0 {
		return value, fmt.Errorf("no balance returned reading balance for: %s", contract)
	}
	return value.SetBytes(response.ConstantResult[0]), nil
}

func (c *Client) GetAccount(address string) (*GetAccountResponse, error) {
	req, err := postRequest(c.Url("wallet/getaccount"), map[string]interface{}{
		"address": address,
		"visible": true,
	})

	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	parsed, err := parseResponse(resp, &GetAccountResponse{})
	if err != nil {
		return nil, err
	}
	err = checkError(parsed.Error)
	if err != nil {
		return parsed, err
	}
	if len(parsed.Address) == 0 {
		return parsed, fmt.Errorf("could not find account: %s", address)
	}

	return parsed, nil
}
