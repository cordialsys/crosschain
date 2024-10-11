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

const TRUSTLINE_DECIMALS = 15
const XRP_NATIVE_DECIMALS = 0

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
	Amount             *Balance        `json:"Amount,omitempty"`
	Destination        string          `json:"Destination,omitempty"`
	Fee                string          `json:"Fee"`
	Flags              int64           `json:"Flags"`
	LastLedgerSequence int64           `json:"LastLedgerSequence"`
	Sequence           int64           `json:"Sequence"`
	SigningPubKey      string          `json:"SigningPubKey"`
	TransactionType    string          `json:"TransactionType"`
	TxnSignature       string          `json:"TxnSignature"`
	Hash               string          `json:"hash"`
	DeliverMax         *Balance        `json:"DeliverMax,omitempty"`
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
	XRPAmount   string  `json:"XRPAmount,omitempty"`
	TokenAmount *Amount `json:"TokenAmount,omitempty"`
}

func (tg *TakeGetsOrPays) UnmarshalJSON(data []byte) error {
	var xrpAmount string
	if err := json.Unmarshal(data, &xrpAmount); err == nil {
		tg.XRPAmount = xrpAmount
		return nil
	}

	var tokenAmount Amount
	if err := json.Unmarshal(data, &tokenAmount); err == nil {
		tg.TokenAmount = &tokenAmount
		return nil
	}

	return fmt.Errorf("TakerGetsOrPays is neither a string nor an Amount")
}

type TransactionMeta struct {
	AffectedNodes     []AffectedNodes `json:"AffectedNodes"`
	TransactionIndex  int64           `json:"TransactionIndex,omitempty"`
	TransactionResult string          `json:"TransactionResult,omitempty"`
	DeliveredAmount   *Balance        `json:"delivered_amount,omitempty"`
}

type XRPNode interface {
	IsValidMovement() bool
	GetAddress(txResponse *TransactionResponse) (xc.Address, error)
	GetContract(txResponse *TransactionResponse) (xc.ContractAddress, error)
	GetAmount() (xc.AmountBlockchain, error)
	IsSource(txResponse *TransactionResponse) (bool, error)
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

type CreatedNodeWrapper struct {
	node *CreatedNode
}

func (mnw *CreatedNodeWrapper) IsValidMovement() bool {
	return mnw.node.LedgerEntryType == "AccountRoot" || mnw.node.LedgerEntryType == "RippleState"
}

func (mnw *CreatedNodeWrapper) GetAddress(txResponse *TransactionResponse) (xc.Address, error) {
	if mnw.node.NewFields == nil {
		return "", fmt.Errorf("empty NewFields in CreatedNode")
	}

	if mnw.node.LedgerEntryType == "AccountRoot" {
		return xc.Address(mnw.node.NewFields.Account), nil

	} else if mnw.node.LedgerEntryType == "RippleState" {
		isSource, fetchIsSourceErr := mnw.IsSource(txResponse)
		if fetchIsSourceErr != nil {
			return "", fetchIsSourceErr
		}

		if isSource {
			if mnw.node.NewFields.HighLimit == nil {
				return "", fmt.Errorf("empty HighLimit in NewFields")
			}

			return xc.Address(mnw.node.NewFields.HighLimit.Issuer), nil
		} else {
			if mnw.node.NewFields.LowLimit == nil {
				return "", fmt.Errorf("empty HighLimit in NewFields")
			}

			return xc.Address(mnw.node.NewFields.LowLimit.Issuer), nil
		}
	}

	return "", fmt.Errorf("unknown node type in CreatedNode")
}

func (mnw *CreatedNodeWrapper) GetContract(txResponse *TransactionResponse) (xc.ContractAddress, error) {
	var (
		contract                  xc.ContractAddress
		finalBalanceHumanReadable xc.AmountHumanReadable
		err                       error
	)

	if mnw.node.NewFields == nil {
		return "", fmt.Errorf("empty NewFields in CreatedNode")
	}

	if mnw.node.NewFields.Balance.XRPAmount != "" {
		contract = ""
	} else {
		finalBalanceHumanReadable, err = xc.NewAmountHumanReadableFromStr(mnw.node.NewFields.Balance.TokenAmount.Value)
		if err != nil {
			return "", err
		}

		finalBalanceBlockchain := finalBalanceHumanReadable.ToBlockchain(6)
		zero := xc.NewAmountBlockchainFromUint64(0)

		if finalBalanceBlockchain.Cmp(&zero) > 0 {
			if mnw.node.NewFields.HighLimit == nil {
				return "", fmt.Errorf("empty HighLimit in NewFields")
			}

			contract = xc.ContractAddress(mnw.node.NewFields.HighLimit.Issuer)
		} else {
			if mnw.node.NewFields.LowLimit == nil {
				return "", fmt.Errorf("empty LowLimit in NewFields")
			}

			contract = xc.ContractAddress(mnw.node.NewFields.LowLimit.Issuer)
		}
	}

	return contract, nil
}

func (mnw *CreatedNodeWrapper) GetAmount() (xc.AmountBlockchain, error) {

	transactedAmount, conversionErr := extractCreatedNodeBalance(mnw.node)
	if conversionErr != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to fetch CreatedNode balance: %s", conversionErr.Error())
	}

	return transactedAmount, nil
}

func (mnw *CreatedNodeWrapper) IsSource(txResponse *TransactionResponse) (bool, error) {
	if mnw.node.LedgerEntryType == "AccountRoot" {
		if mnw.node.NewFields == nil {
			return false, fmt.Errorf("empty NewFields in CreatedNode")
		}

		if mnw.node.NewFields.Account != txResponse.Result.Account {
			return false, nil
		} else {
			return true, nil
		}

	} else if mnw.node.LedgerEntryType == "RippleState" {
		finalBalanceHumanReadable, err := xc.NewAmountHumanReadableFromStr(mnw.node.NewFields.Balance.TokenAmount.Value)
		if err != nil {
			return false, err
		}

		finalBalanceBlockchain := finalBalanceHumanReadable.ToBlockchain(6)
		zero := xc.NewAmountBlockchainFromUint64(0)

		if finalBalanceBlockchain.Cmp(&zero) > 0 {
			if mnw.node.NewFields.HighLimit == nil {
				return false, fmt.Errorf("empty HighLimit in NewFields")
			}

			return false, nil
		} else {
			if mnw.node.NewFields.LowLimit == nil {
				return false, fmt.Errorf("empty LowLimit in NewFields")
			}

			return true, nil
		}
	}

	return false, nil
}

type CreatedNode struct {
	LedgerEntryType string     `json:"LedgerEntryType"`
	LedgerIndex     string     `json:"LedgerIndex"`
	NewFields       *NewFields `json:"NewFields"`
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

type ModifiedNodeWrapper struct {
	node *ModifiedNode
}

func (mnw *ModifiedNodeWrapper) IsValidMovement() bool {
	if mnw.node.LedgerEntryType == "AccountRoot" {

		if mnw.node.FinalFields == nil && mnw.node.PreviousFields == nil {
			return false
		}

		return true
	}

	if mnw.node.LedgerEntryType == "RippleState" {
		return true
	}

	return false
}

func (mnw *ModifiedNodeWrapper) GetAddress(txResponse *TransactionResponse) (xc.Address, error) {
	if mnw.node.FinalFields == nil {
		return "", fmt.Errorf("empty FinalFields in ModifiedNode")
	}

	if mnw.node.LedgerEntryType == "AccountRoot" {
		return xc.Address(mnw.node.FinalFields.Account), nil

	} else if mnw.node.LedgerEntryType == "RippleState" {
		isSource, fetchIsSourceErr := mnw.IsSource(txResponse)
		if fetchIsSourceErr != nil {
			return "", fetchIsSourceErr
		}

		if isSource {
			if mnw.node.FinalFields.LowLimit == nil {
				return "", fmt.Errorf("empty HighLimit in FinalFields")
			}

			return xc.Address(mnw.node.FinalFields.LowLimit.Issuer), nil
		} else {
			if mnw.node.FinalFields.HighLimit == nil {
				return "", fmt.Errorf("empty HighLimit in FinalFields")
			}

			return xc.Address(mnw.node.FinalFields.HighLimit.Issuer), nil
		}
	}

	return "", fmt.Errorf("unknown node type in ModifiedNode")
}

func (mnw *ModifiedNodeWrapper) GetContract(txResponse *TransactionResponse) (xc.ContractAddress, error) {
	var (
		contract                  xc.ContractAddress
		finalBalanceHumanReadable xc.AmountHumanReadable
		err                       error
	)

	if mnw.node.FinalFields == nil {
		return "", fmt.Errorf("empty FinalFields in ModifiedNode")
	}

	if mnw.node.FinalFields.Balance.XRPAmount != "" {
		contract = ""
	} else {
		finalBalanceHumanReadable, err = xc.NewAmountHumanReadableFromStr(mnw.node.FinalFields.Balance.TokenAmount.Value)
		if err != nil {
			return "", err
		}

		finalBalanceBlockchain := finalBalanceHumanReadable.ToBlockchain(6)
		zero := xc.NewAmountBlockchainFromUint64(0)

		if finalBalanceBlockchain.Cmp(&zero) > 0 {
			if mnw.node.FinalFields.HighLimit == nil {
				return "", fmt.Errorf("empty HighLimit in FinalFields")
			}

			contract = xc.ContractAddress(mnw.node.FinalFields.HighLimit.Issuer)
		} else {
			if mnw.node.FinalFields.LowLimit == nil {
				return "", fmt.Errorf("empty LowLimit in FinalFields")
			}

			contract = xc.ContractAddress(mnw.node.FinalFields.LowLimit.Issuer)
		}
	}

	return contract, nil
}

func (mnw *ModifiedNodeWrapper) GetAmount() (xc.AmountBlockchain, error) {
	transactedAmount, conversionErr := ExtractModifiedNodeBalance(mnw.node)
	if conversionErr != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to fetch ModifiedNode balance: %s", conversionErr.Error())
	}

	return transactedAmount, nil
}

func (mnw *ModifiedNodeWrapper) IsSource(txResponse *TransactionResponse) (bool, error) {
	if mnw.node.LedgerEntryType == "AccountRoot" {
		if mnw.node.FinalFields == nil {
			return false, fmt.Errorf("empty FinalField in ModifiedNode")
		}

		if mnw.node.FinalFields.Account != txResponse.Result.Account {
			return false, nil
		} else {
			return true, nil
		}

	} else if mnw.node.LedgerEntryType == "RippleState" {
		finalBalanceHumanReadable, err := xc.NewAmountHumanReadableFromStr(mnw.node.FinalFields.Balance.TokenAmount.Value)
		if err != nil {
			return false, err
		}

		finalBalanceBlockchain := finalBalanceHumanReadable.ToBlockchain(6)
		zero := xc.NewAmountBlockchainFromUint64(0)

		if finalBalanceBlockchain.Cmp(&zero) > 0 {
			if mnw.node.FinalFields.HighLimit == nil {
				return false, fmt.Errorf("empty HighLimit in FinalFields")
			}

			return true, nil
		} else {
			if mnw.node.FinalFields.LowLimit == nil {
				return false, fmt.Errorf("empty LowLimit in FinalFields")
			}

			return false, nil
		}
	}

	return false, nil
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
	TokenAmount *Amount `json:"TokenAmount,omitempty"`
}

// UnmarshalJSON is the custom unmarshal method for Balance
func (b *Balance) UnmarshalJSON(data []byte) error {
	var xrpAmount string
	if err := json.Unmarshal(data, &xrpAmount); err == nil {
		b.XRPAmount = xrpAmount
		return nil
	}

	var tokenAmount Amount
	if err := json.Unmarshal(data, &tokenAmount); err == nil {
		b.TokenAmount = &tokenAmount
		return nil
	}

	return fmt.Errorf("balance is neither a string nor an Amount")
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
		var xrpNode XRPNode

		if node.ModifiedNode != nil {
			xrpNode = &ModifiedNodeWrapper{
				node: node.ModifiedNode,
			}
		} else if node.CreatedNode != nil {
			xrpNode = &CreatedNodeWrapper{
				node: node.CreatedNode,
			}
		}

		if xrpNode == nil {
			continue
		}

		// Check to see if node LedgerEntryType is of a valid type.
		if !xrpNode.IsValidMovement() {
			continue
		}

		// Fetch address, contract and amount
		address, fetchAddressErr := xrpNode.GetAddress(&txResponse)
		if fetchAddressErr != nil {
			return xclient.TxInfo{}, fetchAddressErr
		}

		contract, fetchContractErr := xrpNode.GetContract(&txResponse)
		if fetchContractErr != nil {
			return xclient.TxInfo{}, fetchContractErr
		}

		amount, fetchAmountErr := xrpNode.GetAmount()
		if fetchAmountErr != nil {
			return xclient.TxInfo{}, fetchAmountErr
		}

		isSource, fetchIsSourceErr := xrpNode.IsSource(&txResponse)
		if fetchIsSourceErr != nil {
			return xclient.TxInfo{}, fetchIsSourceErr
		}

		if isSource {
			tf.AddSource(
				address,
				contract,
				amount,
				nil,
			)
		} else {
			tf.AddDestination(
				address,
				contract,
				amount,
				nil,
			)
		}

	}

	txInfo.AddTransfer(tf)

	txInfo.Fees = txInfo.CalculateFees()

	return txInfo, nil
}

func extractCreatedNodeBalance(node *CreatedNode) (xc.AmountBlockchain, error) {
	var (
		newBalance xc.AmountHumanReadable
		decimals   int32
		err        error
	)

	if node.NewFields == nil {
		return xc.AmountBlockchain{}, fmt.Errorf("NewFields is empty")
	}

	if node.NewFields.Balance.XRPAmount != "" {
		decimals = XRP_NATIVE_DECIMALS
		newBalance, err = xc.NewAmountHumanReadableFromStr(node.NewFields.Balance.XRPAmount)
		if err != nil {
			return xc.AmountBlockchain{}, err
		}
	} else {
		decimals = TRUSTLINE_DECIMALS
		newBalance, err = xc.NewAmountHumanReadableFromStr(node.NewFields.Balance.TokenAmount.Value)
		if err != nil {
			return xc.AmountBlockchain{}, err
		}
	}

	return newBalance.ToBlockchain(decimals), nil
}

func ExtractModifiedNodeBalance(node *ModifiedNode) (xc.AmountBlockchain, error) {
	var (
		finalBalanceHumanReadable, previousBalanceHumanReadable xc.AmountHumanReadable
		finalFields, previousBalance                            xc.AmountBlockchain
		decimals                                                int32
		parseErr                                                error
	)

	if node.FinalFields == nil || node.PreviousFields == nil {
		return xc.AmountBlockchain{}, fmt.Errorf("FinalFields is empty")
	}

	if node.FinalFields.Balance.XRPAmount != "" {
		decimals = XRP_NATIVE_DECIMALS
		finalBalanceHumanReadable, parseErr = xc.NewAmountHumanReadableFromStr(node.FinalFields.Balance.XRPAmount)

		finalFields = finalBalanceHumanReadable.ToBlockchain(decimals)
	} else {
		decimals = TRUSTLINE_DECIMALS
		finalBalanceHumanReadable, parseErr = xc.NewAmountHumanReadableFromStr(node.FinalFields.Balance.TokenAmount.Value)
		finalFields = finalBalanceHumanReadable.ToBlockchain(decimals)
	}
	if parseErr != nil {
		return xc.AmountBlockchain{}, parseErr
	}

	if node.PreviousFields.Balance.XRPAmount != "" {
		previousBalanceHumanReadable, parseErr = xc.NewAmountHumanReadableFromStr(node.PreviousFields.Balance.XRPAmount)
		previousBalance = previousBalanceHumanReadable.ToBlockchain(decimals)
	} else {
		previousBalanceHumanReadable, parseErr = xc.NewAmountHumanReadableFromStr(node.PreviousFields.Balance.TokenAmount.Value)
		previousBalance = previousBalanceHumanReadable.ToBlockchain(decimals)
	}
	if parseErr != nil {
		return xc.AmountBlockchain{}, parseErr
	}

	transactedAmount := previousBalance.Sub(&finalFields)

	return transactedAmount.Abs(), nil
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
	return humanReadbleBalance.ToBlockchain(TRUSTLINE_DECIMALS), nil
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
