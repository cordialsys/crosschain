package client

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/xrp/tx"
	xrptxinput "github.com/cordialsys/crosschain/chain/xrp/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

// Client for XRP
type Client struct {
	Url        string
	HttpClient *http.Client
	Asset      xc.ITask
}

var _ xclient.FullClient = &Client{}

// NewClient returns a new JSON-RPC Client to the XRP node
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()

	return &Client{
		Url:        cfg.URL,
		HttpClient: http.DefaultClient,
		Asset:      cfgI,
	}, nil
}

const MethodPost string = "POST"

type LedgerIndex string

const Validated LedgerIndex = "validated"
const Current LedgerIndex = "current"

type AccountInfoRequest struct {
	Method string                  `json:"method"`
	Params []AccountInfoParamEntry `json:"params"`
}

type AccountInfoParamEntry struct {
	Account     xc.Address  `json:"account"`
	LedgerIndex LedgerIndex `json:"ledger_index"`
}

type AccountLinesRequest struct {
	Method string                   `json:"method"`
	Params []AccountLinesParamEntry `json:"params"`
}

type AccountLinesParamEntry struct {
	Account xc.Address `json:"account"`
}

type TransactionRequest struct {
	Method string                  `json:"method"`
	Params []TransactionParamEntry `json:"params"`
}

type TransactionParamEntry struct {
	Transaction xc.TxHash `json:"transaction"`
	Binary      bool      `json:"binary"`
}

type LedgerRequest struct {
	Method string             `json:"method"`
	Params []LedgerParamEntry `json:"params"`
}

type LedgerParamEntry struct {
	LedgerIndex  LedgerIndex `json:"ledger_index"`
	Transactions bool        `json:"transactions"`
	Expand       bool        `json:"expand"`
	OwnerFunds   bool        `json:"owner_funds"`
}

type SubmitRequest struct {
	Method string             `json:"method"`
	Params []SubmitParamEntry `json:"params"`
}

type SubmitParamEntry struct {
	TxBlob string `json:"tx_blob"`
}

type SubmitResponse struct {
	Result SubmitResult `json:"result"`
}

type SubmitResult struct {
	Accepted                 bool               `json:"accepted"`
	AccountSequenceAvailable int64              `json:"account_sequence_available"`
	AccountSequenceNext      int64              `json:"account_sequence_next"`
	Applied                  bool               `json:"applied"`
	Broadcast                bool               `json:"broadcast"`
	EngineResult             string             `json:"engine_result"`
	EngineResultCode         int64              `json:"engine_result_code"`
	EngineResultMessage      string             `json:"engine_result_message"`
	Kept                     bool               `json:"kept"`
	OpenLedgerCost           string             `json:"open_ledger_cost"`
	Queued                   bool               `json:"queued"`
	TxBlob                   string             `json:"tx_blob"`
	TxJson                   SubmitResultTxJson `json:"tx_json"`
	ValidatedLedgerIndex     int64              `json:"validated_ledger_index"`
	Status                   string             `json:"status"`
}

type SubmitResultTxJson struct {
	Account            string   `json:"Account"`
	Amount             *Balance `json:"Amount"`
	Destination        string   `json:"Destination"`
	Fee                string   `json:"Fee"`
	Flags              int64    `json:"Flags"`
	LastLedgerSequence int64    `json:"LastLedgerSequence"`
	Sequence           int64    `json:"Sequence"`
	SigningPubKey      string   `json:"SigningPubKey"`
	TransactionType    string   `json:"TransactionType"`
	TxnSignature       string   `json:"TxnSignature"`
	Hash               string   `json:"hash"`
}

type LedgerResponse struct {
	Result LedgerResult `json:"result"`
}

type LedgerResult struct {
	Ledger             LedgerInfo `json:"ledger"`
	LedgerCurrentIndex int64      `json:"ledger_current_index"`
	Validated          bool       `json:"validated"`
	Status             string     `json:"status"`
}

type LedgerInfo struct {
	Closed      bool   `json:"closed"`
	LedgerIndex string `json:"ledger_index"`
	ParentHash  string `json:"parent_hash"`
}

type TransactionResponse struct {
	Result TransactionResult `json:"result"`
}

type TransactionResult struct {
	Account            string          `json:"Account"`
	Amount             string          `json:"Amount,omitempty"`
	Destination        string          `json:"Destination,omitempty"`
	Fee                string          `json:"Fee"`
	Flags              int64           `json:"Flags"`
	LastLedgerSequence int64           `json:"LastLedgerSequence"`
	Sequence           int64           `json:"Sequence"`
	SigningPubKey      string          `json:"SigningPubKey"`
	TransactionType    string          `json:"TransactionType"`
	TxnSignature       string          `json:"TxnSignature"`
	Hash               string          `json:"hash"`
	DeliverMax         string          `json:"DeliverMax,omitempty"`
	TakerGets          *TakeGetsOrPays `json:"TakerGets,omitempty"`
	TakerPays          *TakeGetsOrPays `json:"TakerPays,omitempty"`
	CtID               string          `json:"ctid,omitempty"`
	Meta               TransactionMeta `json:"meta"`
	Validated          bool            `json:"validated"`
	Date               int64           `json:"date"`
	LedgerIndex        int64           `json:"ledger_index"`
	InLedger           int64           `json:"inLedger"`
	Status             string          `json:"status"`
}

type TakeGetsOrPays struct {
	XRPAmount   string  `json:"-"`
	TokenAmount *Amount `json:"-"`
}

func (tg *TakeGetsOrPays) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		tg.XRPAmount = str
		return nil
	}

	var amount Amount
	if err := json.Unmarshal(data, &amount); err == nil {
		tg.TokenAmount = &amount
		return nil
	}

	return fmt.Errorf("TakerGets is neither a string nor an Amount")
}

type TransactionMeta struct {
	AffectedNodes     []AffectedNodes `json:"AffectedNodes"`
	TransactionIndex  int64           `json:"TransactionIndex"`
	TransactionResult string          `json:"TransactionResult"`
	DeliveredAmount   string          `json:"delivered_amount,omitempty"`
}

type AffectedNodes struct {
	CreatedNode  *CreatedNode  `json:"CreatedNode,omitempty"`
	ModifiedNode *ModifiedNode `json:"ModifiedNode,omitempty"`
}

// UnmarshalJSON for AffectedNode
func (an *AffectedNodes) UnmarshalJSON(data []byte) error {
	var nodeMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &nodeMap); err != nil {
		return err
	}

	if created, ok := nodeMap["CreatedNode"]; ok {
		var createdNode CreatedNode
		if err := json.Unmarshal(created, &createdNode); err != nil {
			return err
		}
		an.CreatedNode = &createdNode
		return nil
	}

	if modified, ok := nodeMap["ModifiedNode"]; ok {
		var modifiedNode ModifiedNode
		if err := json.Unmarshal(modified, &modifiedNode); err != nil {
			return err
		}
		an.ModifiedNode = &modifiedNode
		return nil
	}

	return fmt.Errorf("unknown node type in AffectedNode")
}

type CreatedNode struct {
	LedgerEntryType string    `json:"LedgerEntryType"`
	LedgerIndex     string    `json:"LedgerIndex"`
	NewFields       NewFields `json:"NewFields"`
}

type NewFields struct {
	Account   string   `json:"Account,omitempty"`
	Balance   *Balance `json:"Balance,omitempty"`
	Sequence  int64    `json:"Sequence,omitempty"`
	Flags     int64    `json:"Flags,omitempty"`
	HighLimit *Amount  `json:"HighLimit,omitempty"`
	LowLimit  *Amount  `json:"LowLimit,omitempty"`
	LowNode   string   `json:"LowNode,omitempty"`
	Owner     string   `json:"Owner,omitempty"`
	RootIndex string   `json:"RootIndex,omitempty"`
}

type ModifiedNode struct {
	FinalFields       *FinalFields    `json:"FinalFields,omitempty"`
	LedgerEntryType   string          `json:"LedgerEntryType"`
	LedgerIndex       string          `json:"LedgerIndex"`
	PreviousFields    *PreviousFields `json:"PreviousFields,omitempty"`
	PreviousTxnID     string          `json:"PreviousTxnID"`
	PreviousTxnLgrSeq int64           `json:"PreviousTxnLgrSeq"`
}

type FinalFields struct {
	Account       string   `json:"Account,omitempty"`
	Balance       *Balance `json:"Balance"`
	Flags         int64    `json:"Flags"`
	OwnerCount    int      `json:"OwnerCount,omitempty"`
	Sequence      int64    `json:"Sequence,omitempty"`
	IndexPrevious string   `json:"IndexPrevious,omitempty"`
	Owner         string   `json:"Owner,omitempty"`
	RootIndex     string   `json:"RootIndex,omitempty"`
	HighLimit     *Amount  `json:"HighLimit,omitempty"`
	HighNode      string   `json:"HighNode,omitempty"`
	LowLimit      *Amount  `json:"LowLimit,omitempty"`
	LowNode       string   `json:"LowNode,omitempty"`
	AMMID         string   `json:"AMMID,omitempty"`
}

type Balance struct {
	XRPAmount   string  `json:"-"`
	TokenAmount *Amount `json:"-"`
}

// UnmarshalJSON is the custom unmarshal method for Balance
func (b *Balance) UnmarshalJSON(data []byte) error {
	// Try to unmarshal the data as a string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		b.XRPAmount = str
		return nil
	}

	// If not a string, try to unmarshal it as an Amount
	var amount Amount
	if err := json.Unmarshal(data, &amount); err == nil {
		b.TokenAmount = &amount
		return nil
	}

	// If neither works, return an error
	return fmt.Errorf("TakerGets is neither a string nor an Amount")
}

type Amount struct {
	Currency string `json:"currency"`
	Issuer   string `json:"issuer"`
	Value    string `json:"value"`
}

type PreviousFields struct {
	Balance    Balance `json:"Balance"`
	OwnerCount int     `json:"OwnerCount,omitempty"`
	Sequence   int64   `json:"Sequence,omitempty"`
}

type AccountLinesResponse struct {
	Result AccountLinesResultDetails `json:"result"`
}

type AccountLinesResultDetails struct {
	LedgerHash  string `json:"LedgerHash"`
	LedgerIndex int64  `json:"LedgerIndex"`
	Validated   bool   `json:"Validated"`
	Status      string `json:"Status"`
	Lines       []Line `json:"lines"`
}

type Line struct {
	Account      string `json:"Account"`
	Balance      string `json:"balance"`
	Currency     string `json:"currency"`
	Limit        string `json:"limit"`
	LimitPeer    string `json:"limit_peer"`
	QualityIn    int    `json:"quality_in"`
	QualityOut   int    `json:"quality_out"`
	NoRipple     bool   `json:"no_ripple"`
	NoRipplePeer bool   `json:"no_ripple_peer"`
}

type AccountInfoResponse struct {
	Result AccountInfoResultDetails `json:"result"`
}

type AccountInfoResultDetails struct {
	AccountData AccountData `json:"account_data"`
}

type AccountData struct {
	Account           string `json:"Account"`
	Balance           string `json:"Balance"`
	Flags             int64  `json:"Flags"`
	LedgerEntryType   string `json:"LedgerEntryType"`
	OwnerCount        int    `json:"OwnerCount"`
	PreviousTxnID     string `json:"PreviousTxnID"`
	PreviousTxnLgrSeq int64  `json:"PreviousTxnLgrSeq"`
	Sequence          int64  `json:"Sequence"`
	Index             string `json:"Index"`
}

func (client *Client) FetchBaseInput(ctx context.Context, args xcbuilder.TransferArgs) (xrptxinput.TxInput, error) {
	txInput := xrptxinput.NewTxInput()

	account := args.GetFrom()

	currentSequence, err := client.getNextValidSeqNumber(account)
	if err != nil {
		return xrptxinput.TxInput{}, err
	}
	currentSequencePtr := *currentSequence
	txInput.Sequence = currentSequencePtr

	ledgerSequence, err := client.getLatestValidatedLedgerSequence()
	if err != nil {
		return xrptxinput.TxInput{}, err
	}
	ledgerSequencePtr := *ledgerSequence
	ledgerOffset := int64(20) // Ledger offset
	lastLedgerSequence := ledgerSequencePtr + ledgerOffset
	txInput.LastLedgerSequence = lastLedgerSequence

	return *txInput, nil
}

// FetchTransferInput returns tx input for a Template tx
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	txInput, err := client.FetchBaseInput(ctx, args)
	if err != nil {
		return nil, err
	}

	return &txInput, nil
}

// Deprecated method - use FetchTransferInput
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	txInput, err := client.FetchTransferInput(ctx, args)
	if err != nil {
		return nil, err
	}

	return txInput, nil
}

// SubmitTx submits a Template tx
func (client *Client) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	serializedTxInputBytes, err := txInput.Serialize()
	if err != nil {
		return err
	}

	serializedTxInputHex := hex.EncodeToString(serializedTxInputBytes)
	serializedTxInputHexBytes := []byte(serializedTxInputHex)

	submitRequest := &SubmitRequest{
		Method: "submit",
		Params: []SubmitParamEntry{
			{
				TxBlob: string(serializedTxInputHexBytes),
			},
		},
	}

	var submitResponse SubmitResponse
	err = client.Send(MethodPost, submitRequest, &submitResponse)
	if err != nil {
		return err
	}

	return nil
}

// FetchTxInfo Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
	legacyTxInfo, err := client.FetchLegacyTxInfo(ctx, txHash)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	// Remap to new tx
	return xclient.TxInfoFromLegacy(client.Asset.GetChain().Chain, legacyTxInfo, xclient.Account), nil
}

// FetchLegacyTxInfo Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {

	txRequest := &TransactionRequest{
		Method: "tx",
		Params: []TransactionParamEntry{
			{
				Transaction: txHash,
				Binary:      false,
			},
		},
	}

	var txResponse TransactionResponse
	err := client.Send(MethodPost, txRequest, &txResponse)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

	ledgerRequest := LedgerRequest{
		Method: "ledger",
		Params: []LedgerParamEntry{
			{
				LedgerIndex: "current",
			},
		},
	}

	var ledgerResponse LedgerResponse
	err = client.Send(MethodPost, ledgerRequest, &ledgerResponse)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

	confirmations := ledgerResponse.Result.LedgerCurrentIndex - txResponse.Result.Sequence

	explorer := client.Asset.GetChain().ExplorerURL + "/tx/" + txResponse.Result.Hash + "?cluster=" + client.Asset.GetChain().Net

	var sources []*xc.LegacyTxInfoEndpoint
	var destinations []*xc.LegacyTxInfoEndpoint

	if txResponse.Result.Destination != "" {
		sources = append(sources, &xc.LegacyTxInfoEndpoint{
			Address: xc.Address(txResponse.Result.Account),
			Amount:  xc.NewAmountBlockchainFromStr(txResponse.Result.Amount),
		})

		destinations = append(destinations, &xc.LegacyTxInfoEndpoint{
			Address: xc.Address(txResponse.Result.Destination),
			Amount:  xc.NewAmountBlockchainFromStr(txResponse.Result.Amount),
		})
	} else {
		if txResponse.Result.TakerGets != nil {
			if txResponse.Result.TakerGets.XRPAmount != "" {
				sources = append(sources, &xc.LegacyTxInfoEndpoint{
					Address: xc.Address(txResponse.Result.Account),
					Amount:  xc.NewAmountBlockchainFromStr(txResponse.Result.TakerGets.XRPAmount),
				})

				for _, node := range txResponse.Result.Meta.AffectedNodes {
					if node.ModifiedNode != nil {
						if node.ModifiedNode.LedgerEntryType == "AccountRoot" {
							var (
								finalBalance, previousBalance int64
								parseIntErr                   error
							)

							if node.ModifiedNode.FinalFields != nil {
								finalBalance, parseIntErr = strconv.ParseInt(node.ModifiedNode.FinalFields.Balance.XRPAmount, 10, 64)
								if parseIntErr != nil {
									return xc.LegacyTxInfo{}, parseIntErr
								}
							} else {
								break
							}

							if node.ModifiedNode.PreviousFields != nil {
								previousBalance, parseIntErr = strconv.ParseInt(node.ModifiedNode.PreviousFields.Balance.XRPAmount, 10, 64)
								if parseIntErr != nil {
									return xc.LegacyTxInfo{}, parseIntErr
								}
							} else {
								break
							}

							transactedAmount := finalBalance - previousBalance

							soldAmount, parseIntErr := strconv.ParseInt(txResponse.Result.TakerGets.XRPAmount, 10, 64)
							if parseIntErr != nil {
								return xc.LegacyTxInfo{}, parseIntErr
							}

							if transactedAmount == soldAmount {
								destinations = append(destinations, &xc.LegacyTxInfoEndpoint{
									Address: xc.Address(node.ModifiedNode.FinalFields.Account),
									Amount:  xc.NewAmountBlockchainFromStr(txResponse.Result.TakerGets.XRPAmount),
								})
							}

						}
					}
				}
			} else {
				amount, err := xc.NewAmountHumanReadableFromStr(txResponse.Result.TakerGets.TokenAmount.Value)
				if err != nil {
					return xc.LegacyTxInfo{}, err
				}
				sources = append(sources, &xc.LegacyTxInfoEndpoint{
					Address: xc.Address(txResponse.Result.Account),
					Amount:  amount.ToBlockchain(15),
					Asset:   txResponse.Result.TakerGets.TokenAmount.Currency,
				})
			}
		}

		if txResponse.Result.TakerPays != nil {
			if txResponse.Result.TakerPays.XRPAmount != "" {
				destinations = append(destinations, &xc.LegacyTxInfoEndpoint{
					Address: xc.Address(txResponse.Result.Account),
					Amount:  xc.NewAmountBlockchainFromStr(txResponse.Result.TakerPays.XRPAmount),
				})
			} else {
				amount, err := xc.NewAmountHumanReadableFromStr(txResponse.Result.TakerPays.TokenAmount.Value)
				if err != nil {
					return xc.LegacyTxInfo{}, err
				}

				sources = append(sources, &xc.LegacyTxInfoEndpoint{
					Address: xc.Address(txResponse.Result.TakerPays.TokenAmount.Issuer),
					Amount:  amount.ToBlockchain(15),
					Asset:   txResponse.Result.TakerPays.TokenAmount.Currency,
				})

				for _, node := range txResponse.Result.Meta.AffectedNodes {
					if node.ModifiedNode != nil {
						if node.ModifiedNode.LedgerEntryType == "RippleState" {
							var (
								finalBalance, previousBalance, soldAmount float64
								parseIntErr                               error
							)

							if node.ModifiedNode.FinalFields != nil {
								finalBalance, parseIntErr = strconv.ParseFloat(node.ModifiedNode.FinalFields.Balance.TokenAmount.Value, 10)
								if parseIntErr != nil {
									return xc.LegacyTxInfo{}, parseIntErr
								}
							} else {
								break
							}

							if node.ModifiedNode.PreviousFields != nil {
								previousBalance, parseIntErr = strconv.ParseFloat(node.ModifiedNode.PreviousFields.Balance.TokenAmount.Value, 10)
								if parseIntErr != nil {
									return xc.LegacyTxInfo{}, parseIntErr
								}
							} else {
								break
							}

							transactedAmount := finalBalance - previousBalance

							soldAmount, parseIntErr = strconv.ParseFloat(txResponse.Result.TakerPays.TokenAmount.Value, 10)
							if parseIntErr != nil {
								return xc.LegacyTxInfo{}, parseIntErr
							}

							if transactedAmount == soldAmount {
								destinations = append(destinations, &xc.LegacyTxInfoEndpoint{
									Address: xc.Address(node.ModifiedNode.FinalFields.Account),
									Amount:  xc.NewAmountBlockchainFromStr(txResponse.Result.TakerGets.XRPAmount),
								})
							}

						}
					}
				}

			}
		}
	}

	var status xc.TxStatus
	if txResponse.Result.Status == "success" {
		status = xc.TxStatusSuccess
	} else if txResponse.Result.Status == "error" {
		status = xc.TxStatusFailure
	}

	txInfo := xc.LegacyTxInfo{
		BlockHash:     txResponse.Result.Hash,
		TxID:          txResponse.Result.Hash,
		ExplorerURL:   explorer,
		From:          xc.Address(txResponse.Result.Account),
		To:            xc.Address(txResponse.Result.Destination),
		Amount:        xc.NewAmountBlockchainFromStr(txResponse.Result.Amount),
		Fee:           xc.NewAmountBlockchainFromStr(txResponse.Result.Fee),
		BlockIndex:    txResponse.Result.LedgerIndex,
		BlockTime:     txResponse.Result.Date,
		Confirmations: confirmations,
		Status:        status,
		Sources:       sources,
		Destinations:  destinations,
		Time:          txResponse.Result.Date,
	}

	return txInfo, nil
}

// FetchBalance fetches token balance for a XRP address
func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return client.FetchBalanceForAsset(ctx, address, client.Asset)
}

func (client *Client) FetchBalanceForAsset(ctx context.Context, address xc.Address, assetCfg xc.ITask) (xc.AmountBlockchain, error) {
	switch asset := assetCfg.(type) {
	case *xc.ChainConfig:
		return client.FetchNativeBalance(ctx, address)
	case *xc.TokenAssetConfig:
		return client.fetchContractBalance(ctx, address, asset.Contract)
	default:
		contract := asset.GetContract()
		logrus.WithFields(logrus.Fields{
			"chain":      asset.GetChain().Chain,
			"contract":   contract,
			"asset_type": fmt.Sprintf("%T", asset),
		}).Warn("fetching balance for unknown asset type")
		return client.fetchContractBalance(ctx, address, contract)
	}
}

// FetchNativeBalance fetches account native balance for a XRP address
func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)

	request := AccountInfoRequest{
		Method: "account_info",
		Params: []AccountInfoParamEntry{
			{
				Account:     address,
				LedgerIndex: Validated,
			},
		},
	}

	var accountInfoResponse AccountInfoResponse
	err := client.Send(MethodPost, request, &accountInfoResponse)
	if err != nil {
		return zero, err
	}

	balance := accountInfoResponse.Result.AccountData.Balance
	if balance == "" {
		return zero, fmt.Errorf("empty balance returned for account: %s", address)
	}

	return xc.NewAmountBlockchainFromStr(balance), nil
}

// fetchContractBalance fetches a specific token balance based on received contract for an XRP address
func (client *Client) fetchContractBalance(ctx context.Context, address xc.Address, assetContract string) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)

	asset, contract, err := tx.ExtractAssetAndContract(assetContract)
	if err != nil {
		return zero, fmt.Errorf("failed to parse and extract asset and contract: %w", err)
	}

	request := AccountLinesRequest{
		Method: "account_lines",
		Params: []AccountLinesParamEntry{
			{
				Account: address,
			},
		},
	}

	var accountLinesResponse AccountLinesResponse
	err = client.Send(MethodPost, request, &accountLinesResponse)
	if err != nil {
		return zero, err
	}

	var balance string
	for _, line := range accountLinesResponse.Result.Lines {
		if line.Currency == asset && line.Account == contract {
			balance = line.Balance
		}
	}

	if balance == "" {
		return zero, fmt.Errorf("empty balance returned for account: %s", address)
	}

	humanReadbleBalance, err := xc.NewAmountHumanReadableFromStr(balance)
	if err != nil {
		return zero, fmt.Errorf("failed to parse balance for account: %s", address)
	}
	return humanReadbleBalance.ToBlockchain(15), nil
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

	resp, err := client.HttpClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch balance, HTTP status: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response body: %w", err)
	}

	return nil
}

func (client *Client) getNextValidSeqNumber(address xc.Address) (*int64, error) {
	request := AccountInfoRequest{
		Method: "account_info",
		Params: []AccountInfoParamEntry{
			{
				Account:     address,
				LedgerIndex: Validated,
			},
		},
	}

	var accountInfoResponse AccountInfoResponse
	err := client.Send(MethodPost, request, &accountInfoResponse)
	if err != nil {
		return nil, err
	}

	sequence := accountInfoResponse.Result.AccountData.Sequence
	return &sequence, nil
}

func (client *Client) getLatestValidatedLedgerSequence() (*int64, error) {
	ledgerRequest := LedgerRequest{
		Method: "ledger",
		Params: []LedgerParamEntry{
			{
				LedgerIndex: Current,
			},
		},
	}

	var ledgerResponse LedgerResponse
	err := client.Send(MethodPost, ledgerRequest, &ledgerResponse)
	if err != nil {
		return nil, err
	}

	ledgerCurrentIndex := ledgerResponse.Result.LedgerCurrentIndex
	return &ledgerCurrentIndex, nil
}
