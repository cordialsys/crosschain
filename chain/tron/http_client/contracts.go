package httpclient

import "encoding/json"

type transferContract struct {
	Amount uint64 `json:"amount"`
	Owner  string `json:"owner_address"`
	To     string `json:"to_address"`
}

func (c *ContractData) AsTransferContract() (*transferContract, error) {
	data := &transferContract{}
	bz, err := json.Marshal(c.Parameter.Value)
	if err != nil {
		return data, err
	}

	err = json.Unmarshal(bz, data)
	return data, err
}
