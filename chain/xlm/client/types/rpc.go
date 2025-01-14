package types

import (
	"encoding/base64"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/stellar/go/xdr"
)

const (
	Decimals                     = 7
	AssetTypeLiquidityPoolShares = "liquidity_pool_shares"
	AssetTypeNative              = "native"
	TxStatusPending              = "PENDING"
)

// This type of error can be returned for GET and POST horizon endpoints
type QueryProblem struct {
	Type   string                 `json:"type,omitempty"`
	Title  string                 `json:"title,omitempty"`
	Status int                    `json:"status,omitempty"`
	Detail string                 `json:"detail,omitempty"`
	Extras map[string]interface{} `json:"extras,omitempty"`
}

func (q *QueryProblem) Error() string {
	if q.Type == "" {
		return ""
	}

	return fmt.Sprintf(`Type: "%s", Title: "%s", Status: %d, Detail: "%s", Extras: %v`, q.Type, q.Title, q.Status, q.Detail, q.Extras)
}

type GetTransactionResult struct {
	Id string `json:"id"`
	// Indicates if this transaction was successful or not
	Successful bool   `json:"successful"`
	Hash       string `json:"hash"`
	Ledger     uint64 `json:"ledger,omitempty"`
	// The date this transaction was created.
	// Format: ISO8601 string
	CreatedAt             string `json:"created_at,omitempty"`
	SourceAccount         string `json:"source_account"`
	SourceAccountSequence string `json:"source_account_sequence"`
	FeeAccount            string `json:"fee_account"`
	FeeAccountMuxed       string `json:"fee_account_muxed"`
	FeeAccountMuxedId     string `json:"fee_account_muxed_id"`
	// The fee(in stroops) paid by the source account
	FeeCharged     string `json:"fee_charged"`
	MaxFee         string `json:"max_fee"`
	OperationCount int    `json:"operation_count"`
	// base64 encoded github.com/stellar/go/xdr.TransactionEnvelope XDR binary
	EnvelopeXdr string `json:"envelope_xdr,omitempty"`
	// base64 encoded github.com/stellar/go/xdr.TransactionResult XDR binary
	ResultXdr string `json:"result_xdr,omitempty"`
	// base64 encoded github.com/stellar/go/xdr.TransactionResultMeta XDR binary
	ResultMetaXdr string `json:"result_meta_xdr,omitempty"`
}

type GetLedgerResult struct {
	Id       string `json:"id"`
	Hash     string `json:"hash"`
	Sequence int64  `json:"sequence"`
}

type Balance struct {
	Balance   string `json:"balance"`
	AssetType string `json:"asset_type"`
	// AssetCode is empty when `AssetType == AssetTypeNative`
	AssetCode string `json:"asset_code"`
	// AssetIssuer is empty when `AssetType == AssetTypeNative`
	AssetIssuer string `json:"asset_issuer"`
}

type GetAccountResult struct {
	Sequence string    `json:"sequence"`
	Balances []Balance `json:"balances"`
}

func (account *GetAccountResult) GetNativeBalance() (xc.AmountBlockchain, error) {
	for _, balance := range account.Balances {
		if balance.AssetType != AssetTypeNative {
			continue
		}

		readableAmount, err := xc.NewAmountHumanReadableFromStr(balance.Balance)
		if err != nil {
			return xc.AmountBlockchain{}, fmt.Errorf("failed to read balance decimal: %w", err)
		}

		blockchainAmount := readableAmount.ToBlockchain(Decimals)
		return blockchainAmount, nil
	}

	return xc.NewAmountBlockchainFromUint64(0), nil
}

type Records struct {
	Records []GetLedgerResult `json:"records"`
}

type GetLatestLedgerResult struct {
	Embedded Records `json:"_embedded"`
}

type TransactionResult struct {
	FeeCharged int    `json:"fee_charged,omitempty"`
	Result     string `json:"result,omitempty"`
}

// AsyncTxSubmissionResponse represents a partial result of the [POST /transactions_async] endpoint.
// This response provides the status of a submitted transaction, which can be in one of several states:
// - DUPLICATE: The transaction is a duplicate and was previously submitted.
// - ERROR: There was an error with the transaction, such as an invalid sequence or failed preconditions.
// - PENDING: The transaction was successfully submitted and is awaiting further processing.

// For more details on the transaction states and submission process, refer to the official Stellar documentation:
// https://developers.stellar.org/docs/data/horizon/api-reference/submit-async-transaction
type AsyncTxSubmissionResponse struct {
	// ErrorResultXdr is initially a Base64-encoded string representing an xdr.TransactionResult.
	// It can be decoded into a TransacionResult object by calling DecodeErrorResultXdr().
	ErrorResultXdr interface{} `json:"errorResultXdr,omitempty"`
	TxStatus       string      `json:"tx_status,omitempty"`
	Hash           string      `json:"hash,omitempty"`
}

var _ error = &AsyncTxSubmissionResponse{}

func (r *AsyncTxSubmissionResponse) Error() string {
	if r.TxStatus == "" {
		return ""
	}

	return fmt.Sprintf("ErrorResultXdr: %+v,TsStatus: %s, Hash: %s", r.ErrorResultXdr, r.TxStatus, r.Hash)
}

func XdrResultCodeToString(code xdr.TransactionResultCode) string {
	switch code {
	case xdr.TransactionResultCodeTxFeeBumpInnerSuccess:
		return "tx_fee_bump_inner_success"
	case xdr.TransactionResultCodeTxFeeBumpInnerFailed:
		return "tx_fee_bump_inner_failed"
	case xdr.TransactionResultCodeTxNotSupported:
		return "tx_not_supported"
	case xdr.TransactionResultCodeTxSuccess:
		return "tx_success"
	case xdr.TransactionResultCodeTxFailed:
		return "tx_failed"
	case xdr.TransactionResultCodeTxTooEarly:
		return "tx_too_early"
	case xdr.TransactionResultCodeTxTooLate:
		return "tx_too_late"
	case xdr.TransactionResultCodeTxMissingOperation:
		return "tx_missing_operation"
	case xdr.TransactionResultCodeTxBadSeq:
		return "tx_bad_seq"
	case xdr.TransactionResultCodeTxBadAuth:
		return "tx_bad_auth"
	case xdr.TransactionResultCodeTxInsufficientBalance:
		return "tx_insufficient_balance"
	case xdr.TransactionResultCodeTxNoAccount:
		return "tx_no_source_account"
	case xdr.TransactionResultCodeTxInsufficientFee:
		return "tx_insufficient_fee"
	case xdr.TransactionResultCodeTxBadAuthExtra:
		return "tx_bad_auth_extra"
	case xdr.TransactionResultCodeTxInternalError:
		return "tx_internal_error"
	case xdr.TransactionResultCodeTxBadSponsorship:
		return "tx_bad_sponsorship"
	case xdr.TransactionResultCodeTxBadMinSeqAgeOrGap:
		return "tx_bad_minseq_age_or_gap"
	default:
		return ""
	}
}

func (response *AsyncTxSubmissionResponse) DecodeErrorResultXdr() error {
	strErrorResultXdr, ok := response.ErrorResultXdr.(string)
	if ok == false {
		return nil
	}
	if strErrorResultXdr == "" {
		return nil
	}

	decodedErr, err := base64.StdEncoding.DecodeString(strErrorResultXdr)
	if err != nil {
		return fmt.Errorf("failed to decode ErrorResultXDR: %w", err)
	}

	var xdrTxResult xdr.TransactionResult
	if err := xdrTxResult.UnmarshalBinary(decodedErr); err != nil {
		return fmt.Errorf("failed to unmarshall ErrorResultXDR: %w", err)
	}

	txResult := TransactionResult{
		FeeCharged: int(xdrTxResult.FeeCharged),
		Result:     XdrResultCodeToString(xdrTxResult.Result.Code),
	}

	response.ErrorResultXdr = txResult
	return nil
}

// Async transaction submission can fail in two ways:
// 1. QueryProblem: Returned when the submission does not follow the endpoint specification.
// 2. AsyncTxSubmissionResult: Returned for submitted transactions, or for invalid transactions.
type AsyncTxSubmissionResult struct {
	QueryProblem
	AsyncTxSubmissionResponse
}

func (result AsyncTxSubmissionResult) IsError() bool {
	return result.TxStatus != TxStatusPending
}

func (r *AsyncTxSubmissionResult) Error() string {
	if r.QueryProblem.Title != "" {
		return r.QueryProblem.Error()
	} else {
		return r.AsyncTxSubmissionResponse.Error()
	}
}
