package types

import (
	"context"

	xc "github.com/cordialsys/crosschain"
)

const BitcoinCashPrefix = "bitcoincash:"

type BlockBookClient interface {
	// Currently the only method we _need_ is ListUtxo
	ListUtxo(ctx context.Context, addr string, confirmed bool) (UtxoResponse, error)

	// Previously used:
	// EstimateFee(ctx context.Context, blocks int) (EstimateFeeResponse, error)
	// LatestStats(ctx context.Context) (StatsResponse, error)
	// SubmitTx(ctx context.Context, txBytes []byte) (string, error)
	// GetTx(ctx context.Context, txHash string) (TransactionResponse, error)
	// GetBlock(ctx context.Context, block uint64) (Block, error)
}

type ErrorResponse struct {
	ErrorMessage string `json:"error"`
	HttpStatus   int    `json:"-"`
}

func (err *ErrorResponse) Error() string {
	return err.ErrorMessage
}

type BlockbookStats struct {
	Coin            string `json:"coin"`
	Host            string `json:"host"`
	Version         string `json:"version"`
	GitCommit       string `json:"gitCommit"`
	BuildTime       string `json:"buildTime"`
	SyncMode        bool   `json:"syncMode"`
	InitialSync     bool   `json:"initialSync"`
	InSync          bool   `json:"inSync"`
	BestHeight      int64  `json:"bestHeight"`
	LastBlockTime   string `json:"lastBlockTime"`
	InSyncMempool   bool   `json:"inSyncMempool"`
	LastMempoolTime string `json:"lastMempoolTime"`
	MempoolSize     int    `json:"mempoolSize"`
	Decimals        int    `json:"decimals"`
	DBSize          int64  `json:"dbSize"`
	About           string `json:"about"`
}

type BackendStats struct {
	Chain           string                 `json:"chain"`
	Blocks          int                    `json:"blocks"`
	Headers         int                    `json:"headers"`
	BestBlockHash   string                 `json:"bestBlockHash"`
	Difficulty      xc.AmountHumanReadable `json:"difficulty"`
	SizeOnDisk      int64                  `json:"sizeOnDisk"`
	Version         string                 `json:"version"`
	Subversion      string                 `json:"subversion"`
	ProtocolVersion string                 `json:"protocolVersion"`
	TimeOffset      int64                  `json:"timeOffset"`
	Warnings        string                 `json:"warnings"`
}

type StatsResponse struct {
	Blockbook BlockbookStats `json:"blockbook"`
	Backend   BackendStats   `json:"backend"`
}

type SubmitResponse struct {
	Result string `json:"result"`
}

type UtxoResponse []Utxo
type Utxo struct {
	TxID          string `json:"txid"`
	Vout          int    `json:"vout"`
	Value         string `json:"value"`
	Confirmations uint64 `json:"confirmations"`
	LockTime      int64  `json:"lockTime"`
	Height        int64  `json:"height"`
}

func (u Utxo) GetValue() uint64 {
	return xc.NewAmountBlockchainFromStr(u.Value).Uint64()
}
func (u Utxo) GetBlock() uint64 {
	if u.Height < 0 {
		return 0
	}
	return uint64(u.Height)
}
func (u Utxo) GetTxHash() string {
	return u.TxID
}
func (u Utxo) GetIndex() uint32 {
	return uint32(u.Vout)
}

type Vin struct {
	TxID      string              `json:"txid"`
	Vout      int                 `json:"vout"`
	Sequence  uint32              `json:"sequence"`
	N         int                 `json:"n"`
	Addresses []string            `json:"addresses"`
	IsAddress bool                `json:"isAddress"`
	Value     xc.AmountBlockchain `json:"value"`
	Hex       string              `json:"hex"`
}

type Vout struct {
	Value     xc.AmountBlockchain `json:"value"`
	N         int                 `json:"n"`
	Hex       string              `json:"hex"`
	Addresses []string            `json:"addresses"`
	IsAddress bool                `json:"isAddress"`
}

type TransactionResponse struct {
	TxID          string `json:"txid"`
	Version       int    `json:"version"`
	Vin           []Vin  `json:"vin"`
	Vout          []Vout `json:"vout"`
	BlockHash     string `json:"blockHash"`
	BlockHeight   int    `json:"blockHeight"`
	Confirmations int    `json:"confirmations"`
	BlockTime     int64  `json:"blockTime"`
	Size          int    `json:"size"`
	Vsize         int    `json:"vsize"`
	Value         string `json:"value"`
	ValueIn       string `json:"valueIn"`
	Fees          string `json:"fees"`
	Hex           string `json:"hex"`
}

type EstimateFeeResponse struct {
	// This is a decimal string.  It is BTC/kilobyte.
	Result string `json:"result"`
}

type BlockHeader struct {
	Page              int                    `json:"page"`
	TotalPages        int                    `json:"totalPages"`
	ItemsOnPage       int                    `json:"itemsOnPage"`
	Hash              string                 `json:"hash"`
	PreviousBlockHash string                 `json:"previousBlockHash"`
	NextBlockHash     string                 `json:"nextBlockHash"`
	Height            int                    `json:"height"`
	Confirmations     int                    `json:"confirmations"`
	Size              int                    `json:"size"`
	Time              int64                  `json:"time"`
	Version           int                    `json:"version"`
	MerkleRoot        string                 `json:"merkleRoot"`
	Nonce             xc.AmountBlockchain    `json:"nonce"`
	Bits              string                 `json:"bits"`
	Difficulty        xc.AmountHumanReadable `json:"difficulty"`
	TxCount           int                    `json:"txCount"`
}

type Block struct {
	BlockHeader
	Txs []Tx     `json:"txs"`
	Tx  []string `json:"tx"`
}

func (b *Block) GetTxIds() []string {
	if len(b.Txs) > 0 {
		ids := make([]string, len(b.Txs))
		for i, tx := range b.Txs {
			ids[i] = tx.TxID
		}
		return ids
	}
	return b.Tx
}

type Tx struct {
	TxID          string `json:"txid"`
	Vin           []Vin  `json:"vin"`
	Vout          []Vout `json:"vout"`
	BlockHash     string `json:"blockHash"`
	BlockHeight   int    `json:"blockHeight"`
	Confirmations int    `json:"confirmations"`
	BlockTime     int64  `json:"blockTime"`
	Value         string `json:"value"`
	ValueIn       string `json:"valueIn"`
	Fees          string `json:"fees"`
}

type AddressResponse struct {
	Page               int      `json:"page"`
	TotalPages         int      `json:"totalPages"`
	ItemsOnPage        int      `json:"itemsOnPage"`
	Address            string   `json:"address"`
	Balance            string   `json:"balance"`
	TotalReceived      string   `json:"totalReceived"`
	TotalSent          string   `json:"totalSent"`
	UnconfirmedBalance string   `json:"unconfirmedBalance"`
	UnconfirmedTxs     int      `json:"unconfirmedTxs"`
	Txs                int      `json:"txs"`
	Txids              []string `json:"txids"`
}
