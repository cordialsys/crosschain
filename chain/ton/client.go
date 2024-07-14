package ton

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/ton/api"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/sirupsen/logrus"
)

// Client for Template
type Client struct {
	Url    string
	Asset  xc.ITask
	ApiKey string
}

var _ xclient.FullClient = &Client{}

// NewClient returns a new Template Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	url := cfgI.GetChain().URL
	url = strings.TrimSuffix(url, "/")
	apiKey := cfgI.GetChain().AuthSecret

	return &Client{url, cfgI, apiKey}, nil
}

func (cli *Client) get(path string, response any) error {
	return cli.send("GET", path, nil, response)
}
func (cli *Client) post(path string, requestBody any, response any) error {
	return cli.send("POST", path, requestBody, response)
}
func (cli *Client) send(method string, path string, requestBody any, response any) error {
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
		request.Header.Add("X-API-Key", cli.ApiKey)
	}
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

	if resp.StatusCode == http.StatusOK {
		if response != nil {
			if err := json.Unmarshal(body, response); err != nil {
				return fmt.Errorf("failed to unmarshal response: %v", err)
			}
		}
		return nil
	} else {
		// Deserialize to ErrorResponse struct for other status codes
		var errorResponse api.ErrorResponse
		logrus.WithField("body", string(body)).Debug("error")
		if err := json.Unmarshal(body, &errorResponse); err != nil {
			return fmt.Errorf("failed to unmarshal error response: %v", err)
		}
		if errorResponse.Error != "" {
			return fmt.Errorf("%s", errorResponse.Error)
		}
		if len(errorResponse.Detail) > 0 {
			return fmt.Errorf("%s: %s", errorResponse.Detail[0].Type, errorResponse.Detail[0].Msg)
		}
		logrus.WithField("body", string(body)).WithField("chain", cli.Asset.GetChain().Chain).Warn("unknown ton error")
		return fmt.Errorf("unknown ton error (%d)", resp.StatusCode)
	}
}

// FetchTxInput returns tx input for a Template tx
func (client *Client) FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// info := &api.MasterChainInfo{}
	// err := client.get("/api/v3/masterchainInfo", info)
	// if err != nil {
	// 	return nil, fmt.Errorf("could not get main chain info: %v", err)
	// }
	var err error
	acc := &api.GetAccountResponse{}
	err = client.get(fmt.Sprintf("/api/v3/account?address=%s", from), acc)
	if err != nil {
		return nil, fmt.Errorf("could not get address info: %v", err)
	}

	getSeqResponse := &api.GetMethodResponse{}

	err = client.post("api/v3/runGetMethod", &api.GetMethodRequest{
		Address: string(from),
		Method:  api.GetSequenceMethod,
		Stack:   []api.StackItem{},
	}, getSeqResponse)
	if err != nil {
		return nil, fmt.Errorf("could not get address sequence: %v", err)
	}
	sequence := uint64(0)
	if getSeqResponse.ExitCode == 0 && len(getSeqResponse.Stack) > 0 {
		// sequence exists, use it
		parsed := xc.NewAmountBlockchainFromStr(getSeqResponse.Stack[0].Value)
		sequence = parsed.Uint64()
	} else {
		// starts at 0 when address isn't initialized yet
	}

	input := &TxInput{
		TxInputEnvelope: NewTxInput().TxInputEnvelope,
		// MasterChainInfo: *info,
		AccountStatus: acc.Status,
		Timestamp:     time.Now().Unix(),
		Sequence:      sequence,
	}

	getAddrResponse := &api.GetMethodResponse{}
	err = client.post("api/v3/runGetMethod", &api.GetMethodRequest{
		Address: string(from),
		Method:  api.GetPublicKeyMethod,
		Stack:   []api.StackItem{},
	}, getAddrResponse)
	if err != nil {
		return nil, fmt.Errorf("could not get address public-key: %v", err)
	}
	if getAddrResponse.ExitCode == 0 && len(getAddrResponse.Stack) > 0 {
		// Set the public key if the account is present on chain.
		// If not, the public key will need to be set by caller.
		input.SetPublicKeyFromStr(getAddrResponse.Stack[0].Value)
	}

	return input, nil
}

// SubmitTx submits a Template tx
func (client *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	bz, err := tx.Serialize()
	if err != nil {
		return err
	}
	bzBase64 := base64.StdEncoding.EncodeToString(bz)
	fmt.Printf("\n%s\n", hex.EncodeToString(bz))
	resp := &api.SubmitMessageResponse{}
	err = client.post("api/v3/message", &api.SubmitMessageRequest{
		Boc: bzBase64,
	}, resp)
	if err != nil {
		return err
	}
	fmt.Println("submitted: ", resp.MessageHash)
	return nil
}

// Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	chainInfo := &api.MasterChainInfo{}
	err := client.get("/api/v3/masterchainInfo", chainInfo)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	transactions := &api.TransactionsData{}
	err = client.get(fmt.Sprintf("api/v3/transactions?hash=%s", txHash), transactions)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	if len(transactions.Transactions) == 0 {
		return xc.LegacyTxInfo{}, fmt.Errorf("no TON transaction found by %s", txHash)
	}
	tx := transactions.Transactions[0]

	sources := []*xc.LegacyTxInfoEndpoint{}
	dests := []*xc.LegacyTxInfoEndpoint{}
	chain := client.Asset.GetChain().Chain

	totalFee := xc.NewAmountBlockchainFromStr(tx.TotalFees)
	memos := []string{}

	for _, msg := range tx.OutMsgs {
		if msg.Bounced != nil && *msg.Bounced {
			// if the message bounced, do no add endpoints
		} else {
			if msg.Destination != nil && *msg.Destination != "" && msg.Value != nil {
				addr, err := ParseAddress(xc.Address(*msg.Destination))
				if err != nil {
					return xc.LegacyTxInfo{}, err
				}
				value := xc.NewAmountBlockchainFromStr(*msg.Value)
				dests = append(dests, &xc.LegacyTxInfoEndpoint{
					Address:         xc.Address(addr.String()),
					ContractAddress: "",
					Amount:          value,
					NativeAsset:     chain,
				})
			}
			if msg.Source != nil && *msg.Source != "" && msg.Value != nil {
				addr, err := ParseAddress(xc.Address(*msg.Source))
				if err != nil {
					return xc.LegacyTxInfo{}, err
				}
				value := xc.NewAmountBlockchainFromStr(*msg.Value)
				sources = append(sources, &xc.LegacyTxInfoEndpoint{
					Address:         xc.Address(addr.String()),
					ContractAddress: "",
					Amount:          value,
					NativeAsset:     chain,
				})
			}
		}
		if msg.MessageContent.Decoded != nil && msg.MessageContent.Decoded.Type == "text_comment" {
			memos = append(memos, msg.MessageContent.Decoded.Comment)
		}
	}

	info := xc.LegacyTxInfo{
		BlockHash:     tx.BlockRef.Shard,
		BlockIndex:    tx.McBlockSeqno,
		BlockTime:     tx.Now,
		Confirmations: chainInfo.Last.Seqno - tx.McBlockSeqno,

		TxID:        tx.Hash,
		ExplorerURL: "",

		Sources:      sources,
		Destinations: dests,
		Memos:        memos,
		Fee:          totalFee,
		From:         "",
		To:           "",
		ToAlt:        "",
		Amount:       xc.AmountBlockchain{},

		// unused fields
		ContractAddress: "",
		FeeContract:     "",
		Time:            0,
		TimeReceived:    0,
	}
	if len(info.Sources) > 0 {
		info.From = info.Sources[0].Address
	}
	if len(info.Destinations) > 0 {
		info.To = info.Destinations[0].Address
		info.Amount = info.Destinations[0].Amount
	}

	return info, nil
}

// Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHash)
	if err != nil {
		return xclient.TxInfo{}, err
	}
	chain := client.Asset.GetChain().Chain

	// remap to new tx
	return xclient.TxInfoFromLegacy(chain, legacyTx, xclient.Account), nil
}

func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	resp := &api.GetAccountResponse{}
	err := client.get(fmt.Sprintf("/api/v3/account?address=%s", address), resp)
	if err != nil {
		return xc.AmountBlockchain{}, err
	}
	return xc.NewAmountBlockchainFromStr(resp.Balance), nil
}

func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	if client.Asset.GetContract() == "" {
		return client.FetchNativeBalance(ctx, address)
	}
	resp := &api.JettonWalletsResponse{}
	err := client.get(fmt.Sprintf("/api/v3/jetton/wallets?owner_address=%s&jetton_address=%s", address, client.Asset.GetContract()), resp)
	if err != nil {
		return xc.AmountBlockchain{}, err
	}
	sum := xc.NewAmountBlockchainFromUint64(0)
	for _, wallet := range resp.JettonWallets {
		bal := xc.NewAmountBlockchainFromStr(wallet.Balance)
		sum = sum.Add(&bal)
	}
	return sum, nil
}
