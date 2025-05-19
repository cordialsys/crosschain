package tx

import (
	"encoding/hex"
	"encoding/json"

	"github.com/kaspanet/kaspad/domain/consensus/model/externalapi"
)

type TransactionMessage struct {
	Transaction *Transaction `json:"transaction"`
	AllowOrphan bool         `json:"allowOrphan"`
}

type Transaction struct {
	Version      uint32               `json:"version"`
	Inputs       []*TransactionInput  `json:"inputs"`
	Outputs      []*TransactionOutput `json:"outputs"`
	LockTime     uint64               `json:"lockTime"`
	SubnetworkId string               `json:"subnetworkId"`
}

type TransactionInput struct {
	PreviousOutpoint *Outpoint `json:"previousOutpoint"`
	SignatureScript  string    `json:"signatureScript"`
	Sequence         uint64    `json:"sequence"`
	SigOpCount       uint32    `json:"sigOpCount"`
}

type Outpoint struct {
	TransactionId string `json:"transactionId"`
	Index         uint32 `json:"index"`
}

type TransactionOutput struct {
	Amount          uint64           `json:"amount"`
	ScriptPublicKey *ScriptPublicKey `json:"scriptPublicKey"`
}

type ScriptPublicKey struct {
	Version         uint32 `json:"version"`
	ScriptPublicKey string `json:"scriptPublicKey"`
}

func serialize(tx *externalapi.DomainTransaction) ([]byte, error) {
	inputs := make([]*TransactionInput, len(tx.Inputs))
	for i, input := range tx.Inputs {
		inputs[i] = &TransactionInput{
			PreviousOutpoint: &Outpoint{
				TransactionId: input.PreviousOutpoint.TransactionID.String(),
				Index:         input.PreviousOutpoint.Index,
			},
			SignatureScript: hex.EncodeToString(input.SignatureScript),
			Sequence:        input.Sequence,
			SigOpCount:      uint32(input.SigOpCount),
		}
	}

	outputs := make([]*TransactionOutput, len(tx.Outputs))
	for i, output := range tx.Outputs {
		outputs[i] = &TransactionOutput{
			Amount: output.Value,
			ScriptPublicKey: &ScriptPublicKey{
				Version:         uint32(output.ScriptPublicKey.Version),
				ScriptPublicKey: hex.EncodeToString(output.ScriptPublicKey.Script),
			},
		}
	}

	transactionMessage := &TransactionMessage{
		Transaction: &Transaction{
			Version:      uint32(tx.Version),
			Inputs:       inputs,
			Outputs:      outputs,
			LockTime:     tx.LockTime,
			SubnetworkId: tx.SubnetworkID.String(),
		},
		AllowOrphan: false,
	}

	jsonBytes, err := json.MarshalIndent(transactionMessage, "", "  ")
	if err != nil {
		return nil, err
	}

	return jsonBytes, nil
}
