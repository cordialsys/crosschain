package types

import (
	xc "github.com/cordialsys/crosschain"
)

const Cardano = "cardano"

type Amount struct {
	Unit     string `json:"unit"`
	Quantity string `json:"quantity"`
}

type GetAddressInfoResponse struct {
	Address string   `json:"address"`
	Amounts []Amount `json:"amount"`
}

type Block struct {
	Time          int64  `json:"time"`
	Height        uint64 `json:"height"`
	Hash          string `json:"hash"`
	Slot          uint64 `json:"slot"`
	Confirmations uint64 `json:"confirmations"`
}

type TransactionInfo struct {
	Hash        string `json:"hash"`
	Block       string `json:"block"`
	BlockHeight uint64 `json:"block_height"`
	BlockTime   uint64 `json:"block_time"`
	Fees        string `json:"fees"`
}

type ProtocolParameters struct {
	FeePerByte       uint64 `json:"min_fee_a"`
	FixedFee         uint64 `json:"min_fee_b"`
	MinUtxoValue     string `json:"min_utxo"`
	CoinsPerUtxoWord string `json:"coins_per_utxo_word"`
}

type Utxo struct {
	Address string   `json:"address"`
	Amounts []Amount `json:"amount"`
	TxHash  string   `json:"tx_hash"`
	Index   uint16   `json:"output_index"`
}

func (u *Utxo) GetAssetAmount(contract xc.ContractAddress) xc.AmountBlockchain {
	for _, amount := range u.Amounts {
		if amount.Unit == string(contract) {
			return xc.NewAmountBlockchainFromStr(amount.Quantity)
		}
	}

	return xc.NewAmountBlockchainFromUint64(0)
}

type TransactionUtxos struct {
	Inputs  []Utxo `json:"inputs"`
	Outputs []Utxo `json:"outputs"`
}
