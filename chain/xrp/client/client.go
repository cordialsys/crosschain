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
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
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
	XRPAmount   string  `json:"XRPAmount"`
	TokenAmount *Amount `json:"TokenAmount"`
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

func (tm *TransactionMeta) GetAffectedNodesByType(affectedNodeType string) []AffectedNodes {
	var filteredNodes []AffectedNodes

	for _, node := range tm.AffectedNodes {
		if node.ModifiedNode != nil && node.ModifiedNode.LedgerEntryType == affectedNodeType {
			filteredNodes = append(filteredNodes, node)
		}
	}

	return filteredNodes
}

type AffectedNodes struct {
	CreatedNode  *CreatedNode  `json:"CreatedNode,omitempty"`
	ModifiedNode *ModifiedNode `json:"ModifiedNode,omitempty"`
	DeletedNode  *DeletedNode  `json:"DeletedNode,omitempty"`
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

	if deleted, ok := nodeMap["DeletedNode"]; ok {
		var deletedNode DeletedNode
		if err := json.Unmarshal(deleted, &deletedNode); err != nil {
			return err
		}
		an.DeletedNode = &deletedNode
		return nil
	}

	return fmt.Errorf("unknown node type in AffectedNode")
}

type DeletedNode struct {
	FinalFields     *FinalFields `json:"FinalFields,omitempty"`
	LedgerEntryType string       `json:"LedgerEntryType,omitempty"`
	LedgerIndex     string       `json:"LedgerIndex,omitempty"`
}

type CreatedNode struct {
	LedgerEntryType string    `json:"LedgerEntryType"`
	LedgerIndex     string    `json:"LedgerIndex"`
	NewFields       NewFields `json:"NewFields"`
}

type NewFields struct {
	IndexPrevious string   `json:"IndexPrevious"`
	Account       string   `json:"Account,omitempty"`
	Balance       *Balance `json:"Balance,omitempty"`
	Sequence      int64    `json:"Sequence,omitempty"`
	Flags         int64    `json:"Flags,omitempty"`
	HighLimit     *Amount  `json:"HighLimit,omitempty"`
	HighNode      string   `json:"HighNode,omitempty"`
	LowLimit      *Amount  `json:"LowLimit,omitempty"`
	LowNode       string   `json:"LowNode,omitempty"`
	Owner         string   `json:"Owner,omitempty"`
	RootIndex     string   `json:"RootIndex,omitempty"`
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
	Account           string         `json:"Account,omitempty"`
	Balance           *Balance       `json:"Balance,omitempty"`
	Flags             int64          `json:"Flags"`
	OwnerCount        int            `json:"OwnerCount,omitempty"`
	Sequence          int64          `json:"Sequence,omitempty"`
	IndexPrevious     string         `json:"IndexPrevious,omitempty"`
	Owner             string         `json:"Owner,omitempty"`
	RootIndex         string         `json:"RootIndex,omitempty"`
	HighLimit         *Amount        `json:"HighLimit,omitempty"`
	HighNode          string         `json:"HighNode,omitempty"`
	LowLimit          *Amount        `json:"LowLimit,omitempty"`
	LowNode           string         `json:"LowNode,omitempty"`
	AMMID             string         `json:"AMMID,omitempty"`
	BookDirectory     string         `json:"BookDirectory,omitempty"`
	BookNode          string         `json:"BookNode,omitempty"`
	OwnerNode         string         `json:"OwnerNode,omitempty"`
	PreviousTxnID     string         `json:"PreviousTxnID,omitempty"`
	PreviousTxnLgrSeq int64          `json:"PreviousTxnLgrSeq,omitempty"`
	TakeGets          TakeGetsOrPays `json:"TakeGets,omitempty"`
	TakerPays         TakeGetsOrPays `json:"TakerPays,omitempty"`
	ExchangeRate      string         `json:"ExchangeRate,omitempty"`
	TakerGetsCurrency string         `json:"TakerGetsCurrency,omitempty"`
	TakerGetsIssuer   string         `json:"TakerGetsIssuer,omitempty"`
	TakerPaysCurrency string         `json:"TakerPaysCurrency,omitempty"`
	TakerPaysIssuer   string         `json:"TakerPaysIssuer,omitempty"`
	IndexNext         string         `json:"IndexNext,omitempty"`
}

type Balance struct {
	XRPAmount   string  `json:"XRPAmount,omitempty"`
	TokenAmount *Amount `json:"Amount,omitempty"`
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
	Balance       Balance `json:"Balance"`
	OwnerCount    int     `json:"OwnerCount,omitempty"`
	Sequence      int64   `json:"Sequence,omitempty"`
	IndexNext     string  `json:"IndexNext,omitempty"`
	IndexPrevious string  `json:"IndexPrevious,omitempty"`
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
	txInfo, err := client.GetTxInfo(ctx, txHash)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	return txInfo, nil
}

func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	panic("implement me")
}

func (client *Client) GetTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
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
		return xclient.TxInfo{}, err
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
		return xclient.TxInfo{}, err
	}

	name := xclient.TransactionName(string(client.Asset.GetChain().Chain) + txResponse.Result.Hash)

	block := xclient.NewBlock(uint64(txResponse.Result.LedgerIndex), txResponse.Result.Hash, time.Unix(txResponse.Result.Date, 0))

	confirmations := ledgerResponse.Result.LedgerCurrentIndex - txResponse.Result.Sequence

	var errMsg *string
	if txResponse.Result.Status == "error" {
		msg := "transaction failed"
		errMsg = &msg
	}

	txInfo := xclient.TxInfo{
		Name:          name,
		Hash:          txResponse.Result.Hash,
		Chain:         client.Asset.GetChain().Chain,
		Block:         block,
		Transfers:     []*xclient.Transfer{},
		Fees:          []*xclient.Balance{},
		Confirmations: uint64(confirmations),
		Error:         errMsg,
	}

	affectedNodes := txResponse.Result.Meta.AffectedNodes
	tf := xclient.NewTransfer(client.Asset.GetChain().Chain)
	for _, node := range affectedNodes {

		if node.ModifiedNode != nil {
			modifiedNode := node.ModifiedNode
			if modifiedNode.LedgerEntryType == "AccountRoot" {

				if node.ModifiedNode.FinalFields == nil {
					continue
				}

				var (
					contract     string
					finalBalance float64
					parseErr     error
				)

				if node.ModifiedNode.FinalFields.Balance.XRPAmount != "" {
					contract = ""
				} else {
					finalBalance, parseErr = strconv.ParseFloat(node.ModifiedNode.FinalFields.Balance.TokenAmount.Value, 10)
					if parseErr != nil {
						return xclient.TxInfo{}, parseErr
					}

					if finalBalance > 0 {
						contract = node.ModifiedNode.FinalFields.HighLimit.Issuer
					} else {
						contract = node.ModifiedNode.FinalFields.LowLimit.Issuer
					}
				}

				convertedTransactedAmount, conversionErr := extractModifiedNodeBalance(node.ModifiedNode)
				if conversionErr != nil {
					return xclient.TxInfo{}, conversionErr
				}

				if modifiedNode.FinalFields.Account != txResponse.Result.Account {
					tf.AddDestination(
						xc.Address(modifiedNode.FinalFields.Account),
						xc.ContractAddress(contract),
						convertedTransactedAmount,
						nil,
					)
				} else {
					tf.AddSource(
						xc.Address(modifiedNode.FinalFields.Account),
						xc.ContractAddress(contract),
						convertedTransactedAmount,
						nil,
					)
				}

			} else if node.ModifiedNode.LedgerEntryType == "RippleState" {

				if node.ModifiedNode.FinalFields == nil {
					continue
				}

				var (
					contract        string
					finalBalance    float64
					transactionType string
					parseErr        error
				)

				if node.ModifiedNode.FinalFields.Balance.XRPAmount != "" {
					contract = ""
				} else {
					finalBalance, parseErr = strconv.ParseFloat(node.ModifiedNode.FinalFields.Balance.TokenAmount.Value, 10)
					if parseErr != nil {
						return xclient.TxInfo{}, parseErr
					}

					if finalBalance > 0 {
						transactionType = "source"
						contract = node.ModifiedNode.FinalFields.HighLimit.Issuer
					} else {
						transactionType = "destination"
						contract = node.ModifiedNode.FinalFields.LowLimit.Issuer
					}
				}

				convertedTransactedAmount, conversionErr := extractModifiedNodeBalance(node.ModifiedNode)
				if conversionErr != nil {
					return xclient.TxInfo{}, conversionErr
				}

				//if modifiedNode.FinalFields.Account != txResponse.Result.Account {
				if transactionType == "destination" {
					tf.AddDestination(
						xc.Address(modifiedNode.FinalFields.LowLimit.Issuer),
						xc.ContractAddress(contract),
						convertedTransactedAmount,
						nil,
					)
				} else if transactionType == "source" {
					tf.AddSource(
						xc.Address(modifiedNode.FinalFields.HighLimit.Issuer),
						xc.ContractAddress(contract),
						convertedTransactedAmount,
						nil,
					)
				}
			}

		} else if node.CreatedNode != nil {

			createdNode := node.CreatedNode
			if createdNode.LedgerEntryType == "AccountRoot" {

				var (
					contract   string
					newBalance float64
					decimals   int32
					//transactionType string
					parseErr error
				)

				if node.CreatedNode.NewFields.Balance.XRPAmount != "" {
					contract = ""
					decimals = 0
					newBalance, parseErr = strconv.ParseFloat(node.CreatedNode.NewFields.Balance.XRPAmount, 10)
					if parseErr != nil {
						return xclient.TxInfo{}, parseErr
					}

				} else {
					decimals = 15
					newBalance, parseErr = strconv.ParseFloat(node.CreatedNode.NewFields.Balance.TokenAmount.Value, 10)
					if parseErr != nil {
						return xclient.TxInfo{}, parseErr
					}

					if newBalance > 0 {
						//transactionType = "source"
						contract = node.CreatedNode.NewFields.HighLimit.Issuer
					} else {
						//transactionType = "destination"
						contract = node.CreatedNode.NewFields.LowLimit.Issuer
					}
				}

				var newBalanceString string
				if decimals == 0 {
					newBalanceString = fmt.Sprintf("%.6f", math.Abs(newBalance))
				} else {
					newBalanceString = fmt.Sprintf("%.15f", math.Abs(newBalance))
				}

				newBalanceString = strings.TrimRight(strings.TrimRight(newBalanceString, "0"), ".")

				convertedTransactedAmount, conversionErr := xc.NewAmountHumanReadableFromStr(newBalanceString)
				if conversionErr != nil {
					return xclient.TxInfo{}, conversionErr
				}

				//if transactionType == "destination" {
				if createdNode.NewFields.Account != "" {
					tf.AddDestination(
						xc.Address(createdNode.NewFields.Account),
						xc.ContractAddress(contract),
						convertedTransactedAmount.ToBlockchain(decimals),
						nil,
					)
				} else {
					tf.AddDestination(
						xc.Address(createdNode.NewFields.LowLimit.Issuer),
						xc.ContractAddress(contract),
						convertedTransactedAmount.ToBlockchain(decimals),
						nil,
					)
				}

			} else if createdNode.LedgerEntryType == "RippleState" {

				var (
					contract        string
					newBalance      float64
					decimals        int32
					transactionType string
					address         xc.Address
					parseErr        error
				)

				if node.CreatedNode.NewFields.Balance.XRPAmount != "" {
					contract = ""
					decimals = 0
					newBalance, parseErr = strconv.ParseFloat(node.CreatedNode.NewFields.Balance.XRPAmount, 10)
					if parseErr != nil {
						return xclient.TxInfo{}, parseErr
					}

				} else {
					decimals = 15
					newBalance, parseErr = strconv.ParseFloat(node.CreatedNode.NewFields.Balance.TokenAmount.Value, 10)
					if parseErr != nil {
						return xclient.TxInfo{}, parseErr
					}

					if newBalance > 0 {
						transactionType = "destination"
						contract = node.CreatedNode.NewFields.HighLimit.Issuer
					} else {
						transactionType = "source"
						contract = node.CreatedNode.NewFields.LowLimit.Issuer
					}
				}

				var newBalanceString string
				if decimals == 0 {
					newBalanceString = fmt.Sprintf("%.6f", math.Abs(newBalance))
				} else {
					newBalanceString = fmt.Sprintf("%.15f", math.Abs(newBalance))
				}

				newBalanceString = strings.TrimRight(strings.TrimRight(newBalanceString, "0"), ".")

				convertedTransactedAmount, conversionErr := xc.NewAmountHumanReadableFromStr(newBalanceString)
				if conversionErr != nil {
					return xclient.TxInfo{}, conversionErr
				}

				if transactionType == "destination" {
					address = xc.Address(createdNode.NewFields.LowLimit.Issuer)
				} else if transactionType == "source" {
					address = xc.Address(createdNode.NewFields.HighLimit.Issuer)
				}

				tf.AddDestination(
					address,
					xc.ContractAddress(contract),
					convertedTransactedAmount.ToBlockchain(decimals),
					nil,
				)
			}
		}
	}

	txInfo.AddTransfer(tf)

	txInfo.Fees = txInfo.CalculateFees()

	return txInfo, nil
}

func extractModifiedNodeBalance(node *ModifiedNode) (xc.AmountBlockchain, error) {
	var (
		finalBalance, previousBalance float64
		decimals                      int32
		parseErr                      error
	)

	if node.FinalFields.Balance.XRPAmount != "" {
		decimals = 0
		finalBalance, parseErr = strconv.ParseFloat(node.FinalFields.Balance.XRPAmount, 10)
	} else {
		decimals = 15
		finalBalance, parseErr = strconv.ParseFloat(node.FinalFields.Balance.TokenAmount.Value, 10)
	}
	if parseErr != nil {
		return xc.AmountBlockchain{}, parseErr
	}

	if node.PreviousFields.Balance.XRPAmount != "" {
		previousBalance, parseErr = strconv.ParseFloat(node.PreviousFields.Balance.XRPAmount, 10)
	} else {
		previousBalance, parseErr = strconv.ParseFloat(node.PreviousFields.Balance.TokenAmount.Value, 10)
	}
	if parseErr != nil {
		return xc.AmountBlockchain{}, parseErr
	}

	transactedAmount := math.Abs(previousBalance - finalBalance)

	var transactedAmountString string
	if decimals == 0 {
		transactedAmountString = fmt.Sprintf("%.6f", transactedAmount)
	} else {
		transactedAmountString = fmt.Sprintf("%.15f", transactedAmount)
	}

	transactedAmountString = strings.TrimRight(strings.TrimRight(transactedAmountString, "0"), ".")

	convertedTransactedAmount, conversionErr := xc.NewAmountHumanReadableFromStr(transactedAmountString)
	if conversionErr != nil {
		return xc.AmountBlockchain{}, conversionErr
	}

	return convertedTransactedAmount.ToBlockchain(decimals), nil
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
