package crosschain

// LegacyTxInfoEndpoint is a unified view of an endpoint (source or destination) in a TxInfo.
type LegacyTxInfoEndpoint struct {
	Address         Address          `json:"address"`
	ContractAddress ContractAddress  `json:"contract,omitempty"`
	Amount          AmountBlockchain `json:"amount"`
	NativeAsset     NativeAsset      `json:"chain"`
	Asset           string           `json:"asset,omitempty"`
	Memo            string           `json:"memo,omitempty"`

	// Set only when there's a contract ID for native asset (and conflicts with our chosen identifier)
	ContractId ContractAddress `json:"contract_id,omitempty"`
}

// LegacyTxInfo is a unified view of common tx info across multiple blockchains. Use it as an example to build your own.
type LegacyTxInfo struct {
	BlockHash       string                  `json:"block_hash"`
	TxID            string                  `json:"tx_id"`
	From            Address                 `json:"from"`
	To              Address                 `json:"to"`
	ToAlt           Address                 `json:"to_alt,omitempty"`
	ContractAddress ContractAddress         `json:"contract,omitempty"`
	Amount          AmountBlockchain        `json:"amount"`
	Fee             AmountBlockchain        `json:"fee"`
	FeeContract     ContractAddress         `json:"fee_contract,omitempty"`
	BlockIndex      int64                   `json:"block_index,omitempty"`
	BlockTime       int64                   `json:"block_time,omitempty"`
	Confirmations   int64                   `json:"confirmations,omitempty"`
	Status          TxStatus                `json:"status"`
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
type StakeEvent interface {
	GetValidator() string
}

func (info *LegacyTxInfo) AddDroppedDestination(dest *LegacyTxInfoEndpoint) {
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
