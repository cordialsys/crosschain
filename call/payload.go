package call

type SomeContractCall struct {
	// All requests hopefully should be setting `contract_id`.
	ContractID string `json:"contract_id"`
}
