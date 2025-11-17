package txinfo

import xc "github.com/cordialsys/crosschain"

// LegacyTxInfoEndpoint is a unified view of an endpoint (source or destination) in a TxInfo.
type LegacyTxInfoEndpoint struct {
	Address         xc.Address          `json:"address"`
	ContractAddress xc.ContractAddress  `json:"contract,omitempty"`
	Amount          xc.AmountBlockchain `json:"amount"`
	NativeAsset     xc.NativeAsset      `json:"chain"`
	Asset           string              `json:"asset,omitempty"`
	Memo            string              `json:"memo,omitempty"`

	// Set only when there's a contract ID for native asset (and conflicts with our chosen identifier)
	ContractId xc.ContractAddress `json:"contract_id,omitempty"`

	// event details, if known
	Event *Event `json:"event,omitempty"`
}

// LegacyTxInfo is a unified view of common tx info across multiple blockchains. Use it as an example to build your own.
type LegacyTxInfo struct {
	BlockHash       string                  `json:"block_hash"`
	TxID            string                  `json:"tx_id"`
	From            xc.Address              `json:"from"`
	To              xc.Address              `json:"to"`
	ToAlt           xc.Address              `json:"to_alt,omitempty"`
	ContractAddress xc.ContractAddress      `json:"contract,omitempty"`
	Amount          xc.AmountBlockchain     `json:"amount"`
	Fee             xc.AmountBlockchain     `json:"fee"`
	FeePayer        xc.Address              `json:"fee_payer,omitempty"`
	FeeContract     xc.ContractAddress      `json:"fee_contract,omitempty"`
	BlockIndex      int64                   `json:"block_index,omitempty"`
	BlockTime       int64                   `json:"block_time,omitempty"`
	Confirmations   int64                   `json:"confirmations,omitempty"`
	Status          xc.TxStatus             `json:"status"`
	Sources         []*LegacyTxInfoEndpoint `json:"sources,omitempty"`
	Destinations    []*LegacyTxInfoEndpoint `json:"destinations,omitempty"`
	Time            int64                   `json:"time,omitempty"`
	TimeReceived    int64                   `json:"time_received,omitempty"`
	// If this transaction failed, this is the reason why.
	Error string `json:"error,omitempty"`
	// to support new TxInfo model, we can't drop "change" btc movements
	droppedBtcDestinations []*LegacyTxInfoEndpoint
	stakeEvents            []StakeEvent
}

// type StakeEvent interface {
// 	GetValidator() string
// }

func (info *LegacyTxInfo) InsertDestinationAtIndex(index int, value *LegacyTxInfoEndpoint) {
	if index < 0 || index > len(info.Destinations) {
		return
	}

	// Create a new slice with the value inserted
	info.Destinations = append(info.Destinations[:index], append([]*LegacyTxInfoEndpoint{value}, info.Destinations[index:]...)...)
}

func (info *LegacyTxInfo) AddDroppedDestination(index int, dest *LegacyTxInfoEndpoint) {
	info.droppedBtcDestinations = append(info.droppedBtcDestinations, dest)
}
func (info *LegacyTxInfo) GetDroppedBtcDestinations() []*LegacyTxInfoEndpoint {
	return info.droppedBtcDestinations
}

func (info *LegacyTxInfo) AddStakeEvent(ev StakeEvent) {
	info.stakeEvents = append(info.stakeEvents, ev)
}
func (info *LegacyTxInfo) ResetStakeEvents() {
	info.stakeEvents = nil
}
func (info *LegacyTxInfo) GetStakeEvents() []StakeEvent {
	return info.stakeEvents
}
