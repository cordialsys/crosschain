package ton

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	tonaddress "github.com/cordialsys/crosschain/chain/ton/address"
	"github.com/cordialsys/crosschain/chain/ton/api"
	tontx "github.com/cordialsys/crosschain/chain/ton/tx"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/sirupsen/logrus"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton/jetton"
	"github.com/xssnick/tonutils-go/tvm/cell"
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
func (client *Client) GetTokenWallet(ctx context.Context, from xc.Address, contract xc.ContractAddress) (xc.Address, error) {
	net := client.Asset.GetChain().Net
	ownerAddr, err := tonaddress.ParseAddress(from, net)
	if err != nil {
		return "", err
	}
	addrCell := cell.BeginCell().MustStoreAddr(ownerAddr).EndCell()
	// .BeginParse()
	getTokenWalletResponse := &api.GetMethodResponse{}
	err = client.post("api/v3/runGetMethod", &api.GetMethodRequest{
		Address: string(client.Asset.GetContract()),
		Method:  api.GetWalletAddressMethod,
		Stack: []api.StackItem{
			{
				Type:  "slice",
				Value: base64.StdEncoding.EncodeToString(addrCell.ToBOC()),
			},
		},
	}, getTokenWalletResponse)
	if err != nil {
		return "", err
	}
	if getTokenWalletResponse.ExitCode != 0 || len(getTokenWalletResponse.Stack) == 0 {
		return "", fmt.Errorf("could not lookup token wallet for %s (%d)", from, getTokenWalletResponse.ExitCode)
	}
	rawBoc := getTokenWalletResponse.Stack[0].Value
	boc, err := base64.RawStdEncoding.DecodeString(rawBoc)
	if err != nil {
		return "", fmt.Errorf("invalid encoding for token-wallet: %v", err)
	}
	tokenCell, err := cell.FromBOC(boc)
	if err != nil {
		return "", fmt.Errorf("invalid boc for token-wallet: %v", err)
	}
	addr, err := tokenCell.BeginParse().LoadAddr()
	if err != nil {
		return "", fmt.Errorf("invalid token-wallet returned for address: %v", err)
	}
	return xc.Address(addr.String()), nil
}

func (client *Client) EstimateMaxFee(ctx context.Context, from xc.Address, to xc.Address, contract string) (uint64, error) {
	net := client.Asset.GetChain().Net
	fromAddr, _ := tonaddress.ParseAddress(from, net)
	toAddr, _ := tonaddress.ParseAddress(to, net)
	amount, _ := tlb.FromNano(big.NewInt(1), int(client.Asset.GetDecimals()))
	example, err := BuildJettonTransfer(10, toAddr, fromAddr, toAddr, amount, tlb.MustFromTON("1.0"), "")
	if err != nil {
		return 0, err
	}
	c, err := tlb.ToCell(example.InternalMessage)
	if err != nil {
		return 0, err
	}

	feeEstimateResp := &api.FeeEstimateResponse{}
	err = client.post("api/v3/estimateFee", &api.FeeEstimateRequest{
		Address: string(from),
		Body:    base64.StdEncoding.EncodeToString(c.ToBOC()),
	}, feeEstimateResp)
	if err != nil {
		return 0, fmt.Errorf("could not estimate fee: %v", err)
	}
	// Multiply up as the fee is often 2-3x different than what's estimated...
	maxFee := (feeEstimateResp.Sum() * 10)
	if maxFee > 0 {
		return uint64(maxFee), nil
	}
	return 0, nil
}

// FetchLegacyTxInput returns tx input for a Template tx
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
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
		AccountStatus:   acc.Status,
		Timestamp:       time.Now().Unix(),
		Sequence:        sequence,
		TonBalance:      xc.NewAmountBlockchainFromStr(acc.Balance),
	}

	if client.Asset.GetContract() != "" {
		input.TokenWallet, err = client.GetTokenWallet(ctx, from, xc.ContractAddress(client.Asset.GetContract()))
		if err != nil {
			return input, err
		}
		maxFee, err := client.EstimateMaxFee(ctx, input.TokenWallet, to, client.Asset.GetContract())
		if err != nil {
			return input, err
		}
		input.EstimatedMaxFee = xc.NewAmountBlockchainFromUint64(maxFee)
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
		err = input.SetPublicKeyFromStr(getAddrResponse.Stack[0].Value)
		if err != nil {
			logrus.WithError(err).Warn("could not set public key from remote")
		}
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
	resp := &api.SubmitMessageResponse{}
	err = client.post("api/v3/message", &api.SubmitMessageRequest{
		Boc: bzBase64,
	}, resp)
	if err != nil {
		return err
	}
	return nil
}

func (client *Client) LookupTransferForTokenWallet(tokenWallet *address.Address) (*api.JettonTransfer, error) {
	resp := &api.JettonTransfersResponse{}
	err := client.get(fmt.Sprintf("/api/v3/jetton/transfers?jetton_wallet=%s&direction=both&limit=1&offset=0&sort=desc", tokenWallet.String()), resp)
	if err != nil {
		return nil, fmt.Errorf("could not resolve token master address: %v", err)
	}
	if len(resp.JettonTransfers) == 0 {
		return nil, fmt.Errorf("could not resolve token master address: no transfer history")
	}
	return &resp.JettonTransfers[0], nil
	// return xc.ContractAddress(masterAddr.String()), nil
}

func (client *Client) ParseJetton(c *cell.Cell, tokenWallet *address.Address, book api.AddressBook) ([]*xc.LegacyTxInfoEndpoint, []*xc.LegacyTxInfoEndpoint, bool, error) {
	net := client.Asset.GetChain().Net
	jettonTfMaybe := &jetton.TransferPayload{}
	err := tlb.LoadFromCell(jettonTfMaybe, c.BeginParse())
	if err != nil {
		// give up here - no jetton movement(s)
		logrus.WithError(err).Debug("no jetton transfer detected")
		return nil, nil, false, nil
	}
	memo, ok := ParseComment(jettonTfMaybe.ForwardPayload)
	// fmt.Println("memo ", memo, ok)
	if !ok {
		memo, _ = ParseComment(jettonTfMaybe.CustomPayload)
	}
	tf, err := client.LookupTransferForTokenWallet(tokenWallet)
	if err != nil {
		return nil, nil, false, err
	}
	masterAddr, err := tonaddress.ParseAddress(xc.Address(tf.JettonMaster), net)
	if err != nil {
		return nil, nil, false, err
	}
	// The native jetton structure is confusingly inconsistent in that it uses the 'tokenWallet' for the sourceAddress,
	// but uses the owner account for the destinationAddress.  But in the /jetton/transfers endpoint, it is reported
	// using the owner address.  So we use that.
	ownerAddr, err := client.substituteOrParse(book, tf.Source)
	if err != nil {
		return nil, nil, false, err
	}

	chain := client.Asset.GetChain().Chain
	amount := xc.AmountBlockchain(*jettonTfMaybe.Amount.Nano())
	sources := []*xc.LegacyTxInfoEndpoint{
		{
			// this is the token wallet of the sender/owner
			Address:         xc.Address(ownerAddr.String()),
			Amount:          amount,
			ContractAddress: xc.ContractAddress(masterAddr.String()),
			NativeAsset:     chain,
			Memo:            memo,
		},
	}

	dests := []*xc.LegacyTxInfoEndpoint{
		{
			// The destination uses the owner account already
			Address:         xc.Address(jettonTfMaybe.Destination.String()),
			Amount:          amount,
			ContractAddress: xc.ContractAddress(masterAddr.String()),
			NativeAsset:     chain,
			Memo:            memo,
		},
	}
	return sources, dests, true, nil
}

// This detects any JettonMessage in the nest of "InternalMessage"
// This may need to be expanded as Jetton transfer could be nested deeper in more 'InternalMessages'
func (client *Client) DetectJettonMovements(tx *api.Transaction, book api.AddressBook) ([]*xc.LegacyTxInfoEndpoint, []*xc.LegacyTxInfoEndpoint, error) {
	boc, err := base64.StdEncoding.DecodeString(tx.InMsg.MessageContent.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid base64: %v", err)
	}
	inMsg, err := cell.FromBOC(boc)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid boc: %v", err)
	}
	internalMsg := &tlb.InternalMessage{}
	nextMsg, err := inMsg.BeginParse().LoadRefCell()
	if err != nil {
		err = tlb.LoadFromCell(internalMsg, inMsg.BeginParse())
	} else {
		err = tlb.LoadFromCell(internalMsg, nextMsg.BeginParse())
	}
	if err != nil {
		return nil, nil, nil
	}
	if internalMsg.DstAddr == nil {
		return nil, nil, nil
	}

	next := internalMsg.Body
	for next != nil {
		sources, dests, ok, err := client.ParseJetton(next, internalMsg.DstAddr, book)
		if err != nil {
			return nil, nil, fmt.Errorf("%v", err)
		}
		if ok {
			return sources, dests, nil
		}
		nextMsg, err := next.BeginParse().LoadRefCell()
		if err != nil {
			break
		} else {
			next = nextMsg
		}
	}

	return nil, nil, nil
}

// Prioritize getting tx by msg-hash as it's deterministic offline.  Fallback to using chain-calculated tx hash.
func (client *Client) FetchTonTxByHash(ctx context.Context, txHash xc.TxHash) (api.Transaction, api.AddressBook, error) {
	transactions := &api.TransactionsData{}

	// Filter by 'in' direction as this matches messages submitted by the user vs "bounced" transactions created by the chain.
	err := client.get(fmt.Sprintf("api/v3/transactionsByMessage?direction=in&msg_hash=%s", url.QueryEscape(string(txHash))), transactions)
	if err != nil {
		return api.Transaction{}, nil, err
	}
	if len(transactions.Transactions) == 0 {
		// try looking up by chain-issued hash
		err = client.get(fmt.Sprintf("api/v3/transactions?hash=%s", url.QueryEscape(string(txHash))), transactions)
		if err != nil {
			return api.Transaction{}, nil, err
		}

		if len(transactions.Transactions) == 0 {
			return api.Transaction{}, nil, fmt.Errorf("no TON transaction found by %s", txHash)
		}
	}
	return transactions.Transactions[0], transactions.AddressBook, nil
}

// TON tends to report addresses using public key only, but unfortunately this is not sufficient to derive
// an address from, as addresses are also based on various metadata fields (testnet, bounce, version, etc).  Thus it's necessary to use the substitution
// table provided in API responses to figure out the correct address.
func (client *Client) substituteOrParse(book api.AddressBook, rawAddr string) (*address.Address, error) {
	net := client.Asset.GetChain().Net
	if realAddr, ok := book[rawAddr]; ok {
		return tonaddress.ParseAddress(xc.Address(realAddr.UserFriendly), net)
	}
	return tonaddress.ParseAddress(xc.Address(rawAddr), net)
}

// Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	chainInfo := &api.MasterChainInfo{}
	err := client.get("/api/v3/masterchainInfo", chainInfo)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

	tx, addrBook, err := client.FetchTonTxByHash(ctx, txHash)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

	sources := []*xc.LegacyTxInfoEndpoint{}
	dests := []*xc.LegacyTxInfoEndpoint{}
	chain := client.Asset.GetChain().Chain

	totalFee := xc.NewAmountBlockchainFromStr(tx.TotalFees)

	for _, msg := range tx.OutMsgs {
		if msg.Bounced != nil && *msg.Bounced {
			// if the message bounced, do no add endpoints
		} else {
			memo := ""
			if msg.MessageContent.Decoded != nil && msg.MessageContent.Decoded.Type == "text_comment" {
				memo = msg.MessageContent.Decoded.Comment
			}
			if msg.Destination != nil && *msg.Destination != "" && msg.Value != nil {
				addr, err := client.substituteOrParse(addrBook, *msg.Destination)
				if err != nil {
					return xc.LegacyTxInfo{}, fmt.Errorf("invalid address %s: %v", *msg.Destination, err)
				}
				value := xc.NewAmountBlockchainFromStr(*msg.Value)
				dests = append(dests, &xc.LegacyTxInfoEndpoint{
					Address:         xc.Address(addr.String()),
					ContractAddress: "",
					Amount:          value,
					NativeAsset:     chain,
					Memo:            memo,
				})
			}
			if msg.Source != nil && *msg.Source != "" && msg.Value != nil {
				addr, err := client.substituteOrParse(addrBook, *msg.Source)
				if err != nil {
					return xc.LegacyTxInfo{}, fmt.Errorf("invalid address %s: %v", *msg.Source, err)
				}
				value := xc.NewAmountBlockchainFromStr(*msg.Value)
				sources = append(sources, &xc.LegacyTxInfoEndpoint{
					Address:         xc.Address(addr.String()),
					ContractAddress: "",
					Amount:          value,
					NativeAsset:     chain,
					Memo:            memo,
				})
			}
		}

	}

	jettonSources, jettonDests, err := client.DetectJettonMovements(&tx, addrBook)
	if err != nil {
		return xc.LegacyTxInfo{}, fmt.Errorf("could not detect jetton movements: %v", err)
	}
	sources = append(sources, jettonSources...)
	dests = append(dests, jettonDests...)
	info := xc.LegacyTxInfo{
		BlockHash:     tx.BlockRef.Shard,
		BlockIndex:    tx.McBlockSeqno,
		BlockTime:     tx.Now,
		Confirmations: chainInfo.Last.Seqno - tx.McBlockSeqno,

		// Use the InMsg hash as this can be determined offline,
		// whereas the tx.Hash is determined by the chain after submitting.
		TxID:        tontx.Normalize(tx.InMsg.Hash),
		ExplorerURL: "",

		Sources:      sources,
		Destinations: dests,
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
