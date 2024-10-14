package types

import (
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/address/contract"
)

const TRUSTLINE_DECIMALS = 15
const XRP_NATIVE_DECIMALS = 6

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

// The contract and recipient could be in other high or low limit.
// See https://xrpl.org/docs/references/protocol/ledger-data/ledger-entry-types/ripplestate
func DeduceContractFrom(balance *Balance, lowLimit *Amount, highLimit *Amount) (xc.ContractAddress, error) {
	if balance.XRPAmount != "" {
		// this is a native balance, contract is ""
		return "", nil
	}
	finalBalanceHumanReadable, err := xc.NewAmountHumanReadableFromStr(balance.TokenAmount.Value)
	if err != nil {
		return "", err
	}
	symbol := highLimit.Currency

	finalBalanceBlockchain := finalBalanceHumanReadable.ToBlockchain(6)
	zero := xc.NewAmountBlockchainFromUint64(0)

	if finalBalanceBlockchain.Cmp(&zero) > 0 {
		if highLimit == nil {
			return "", fmt.Errorf("empty HighLimit in FinalFields")
		}

		return contract.NewContract(symbol, highLimit.Issuer), nil
	} else {
		if lowLimit == nil {
			return "", fmt.Errorf("empty LowLimit in FinalFields")
		}

		return contract.NewContract(symbol, lowLimit.Issuer), nil
	}
}

func (fields *NewFields) GetContract() (xc.ContractAddress, error) {
	return DeduceContractFrom(fields.Balance, fields.LowLimit, fields.HighLimit)
}

func (fields *FinalFields) GetContract() (xc.ContractAddress, error) {
	return DeduceContractFrom(fields.Balance, fields.LowLimit, fields.HighLimit)
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
