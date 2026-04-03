package tx

import (
	"encoding/json"
	"fmt"

	"github.com/cordialsys/crosschain/chain/canton/tx_input"
)

const (
	TxTypeTransfer      = "transfer"
	TxTypeCreateAccount = "create_account"
)

type Metadata struct {
	TxType string `json:"tx_type"`

	CreateAccount *CreateAccountMetadata `json:"create_account,omitempty"`
}

type CreateAccountMetadata struct {
	Stage string `json:"stage"`

	PartyID              string   `json:"party_id"`
	PublicKeyFingerprint string   `json:"public_key_fingerprint,omitempty"`
	TopologyTransactions [][]byte `json:"topology_transactions,omitempty"`

	SetupProposalAcceptInput *tx_input.CreateAccountAcceptInput `json:"setup_proposal_accept_input,omitempty"`
}

func ParseMetadata(data []byte) (*Metadata, error) {
	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func (m *Metadata) Bytes() ([]byte, error) {
	return json.Marshal(m)
}

func NewTransferMetadata() *Metadata {
	return &Metadata{
		TxType: TxTypeTransfer,
	}
}

func NewCreateAccountMetadata(input *tx_input.CreateAccountInput) *Metadata {
	if input == nil {
		return &Metadata{TxType: TxTypeCreateAccount}
	}
	return &Metadata{
		TxType: TxTypeCreateAccount,
		CreateAccount: &CreateAccountMetadata{
			Stage:                    input.Stage,
			PartyID:                  input.PartyID,
			PublicKeyFingerprint:     input.PublicKeyFingerprint,
			TopologyTransactions:     cloneMetadataBytes2D(input.TopologyTransactions),
			SetupProposalAcceptInput: input.SetupProposalAcceptInput.Clone(),
		},
	}
}

func (m *Metadata) CreateAccountInput(signature []byte) (*tx_input.CreateAccountInput, error) {
	if m == nil || m.CreateAccount == nil {
		return nil, fmt.Errorf("missing Canton create-account metadata")
	}
	input := &tx_input.CreateAccountInput{
		Stage:                    m.CreateAccount.Stage,
		PartyID:                  m.CreateAccount.PartyID,
		PublicKeyFingerprint:     m.CreateAccount.PublicKeyFingerprint,
		TopologyTransactions:     cloneMetadataBytes2D(m.CreateAccount.TopologyTransactions),
		SetupProposalAcceptInput: m.CreateAccount.SetupProposalAcceptInput.Clone(),
		Signature:                append([]byte(nil), signature...),
	}
	if err := input.VerifySignaturePayloads(); err != nil {
		return nil, fmt.Errorf("invalid create-account metadata payload: %w", err)
	}
	return input, nil
}

func cloneMetadataBytes2D(values [][]byte) [][]byte {
	if len(values) == 0 {
		return nil
	}
	cloned := make([][]byte, len(values))
	for i, value := range values {
		cloned[i] = append([]byte(nil), value...)
	}
	return cloned
}
