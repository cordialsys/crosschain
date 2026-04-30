package call

type OfferAcceptCall struct {
	ContractID string `json:"contract_id"`
}

type SettlementCompleteCall struct {
	ContractID string `json:"contract_id"`
}
