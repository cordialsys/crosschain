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

// ServerInfoRequest is used to query the rippled `server_info` endpoint.  We
// use it to read the network's current base reserve and per-object reserve,
// which determine when AccountDelete is required to sweep an account and how
// much is burned by the AccountDelete itself.
type ServerInfoRequest struct {
	Method string             `json:"method"`
	Params []ServerInfoParams `json:"params"`
}

type ServerInfoParams struct{}

type ServerInfoResponse struct {
	Result ServerInfoResult `json:"result"`
}

type ServerInfoResult struct {
	Info ServerInfo `json:"info"`
}

type ServerInfo struct {
	ValidatedLedger ValidatedLedgerInfo `json:"validated_ledger"`
}

// XRP-denominated reserves (e.g. 1.0, 0.2). Convert to drops by multiplying
// by 1_000_000 (XRP_NATIVE_DECIMALS).
type ValidatedLedgerInfo struct {
	BaseFeeXRP     float64 `json:"base_fee_xrp"`
	ReserveBaseXRP float64 `json:"reserve_base_xrp"`
	ReserveIncXRP  float64 `json:"reserve_inc_xrp"`
}
