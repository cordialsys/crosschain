package types

const (
	AssetTypeLiquidityPoolShares string = "liquidity_pool_shares"
)

type GetTransactionResult struct {
	Id string `json:"id"`
	// Indicates if this transaction was successful or ot
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
	Sequence uint64 `json:"sequence"`
}

type Balance struct {
	Balance   string `json:"balance"`
	AssetType string `json:"asset_type"`
	AssetCode string `json:"asset_code"`
}

type GetAccountResult struct {
	Balances []Balance `json:"balances"`
}
