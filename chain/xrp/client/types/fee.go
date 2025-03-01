package types

import (
	xc "github.com/cordialsys/crosschain"
)

type FeeRequest struct {
	Method string      `json:"method"`
	Params []FeeParams `json:"params"`
}

type FeeParams struct {
}

type FeeResponse struct {
	Result FeeResult `json:"result"`
}

type FeeResult struct {
	CurrentLedgerSize  string    `json:"current_ledger_size"`
	CurrentQueueSize   string    `json:"current_queue_size"`
	Drops              FeeDrops  `json:"drops"`
	ExpectedLedgerSize string    `json:"expected_ledger_size"`
	LedgerCurrentIndex int64     `json:"ledger_current_index"`
	Levels             FeeLevels `json:"levels"`
	MaxQueueSize       string    `json:"max_queue_size"`
	Status             string    `json:"status"`
}

type FeeDrops struct {
	BaseFee       xc.AmountBlockchain `json:"base_fee"`
	MedianFee     xc.AmountBlockchain `json:"median_fee"`
	MinimumFee    xc.AmountBlockchain `json:"minimum_fee"`
	OpenLedgerFee xc.AmountBlockchain `json:"open_ledger_fee"`
}

type FeeLevels struct {
	MedianLevel     xc.AmountBlockchain `json:"median_level"`
	MinimumLevel    xc.AmountBlockchain `json:"minimum_level"`
	OpenLedgerLevel xc.AmountBlockchain `json:"open_ledger_level"`
	ReferenceLevel  xc.AmountBlockchain `json:"reference_level"`
}
