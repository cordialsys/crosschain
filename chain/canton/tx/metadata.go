package tx

import (
	"encoding/json"
	"fmt"

	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	"github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive"
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

	SetupProposalPreparedTransaction []byte                           `json:"setup_proposal_prepared_transaction,omitempty"`
	SetupProposalHash                []byte                           `json:"setup_proposal_hash,omitempty"`
	SetupProposalHashing             interactive.HashingSchemeVersion `json:"setup_proposal_hashing,omitempty"`
	SetupProposalSubmissionID        string                           `json:"setup_proposal_submission_id,omitempty"`
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
			Stage:                            input.Stage,
			PartyID:                          input.PartyID,
			PublicKeyFingerprint:             input.PublicKeyFingerprint,
			TopologyTransactions:             cloneMetadataBytes2D(input.TopologyTransactions),
			SetupProposalPreparedTransaction: append([]byte(nil), input.SetupProposalPreparedTransaction...),
			SetupProposalHash:                append([]byte(nil), input.SetupProposalHash...),
			SetupProposalHashing:             input.SetupProposalHashing,
			SetupProposalSubmissionID:        input.SetupProposalSubmissionID,
		},
	}
}

func (m *Metadata) CreateAccountInput(signature []byte) (*tx_input.CreateAccountInput, error) {
	if m == nil || m.CreateAccount == nil {
		return nil, fmt.Errorf("missing Canton create-account metadata")
	}
	input := &tx_input.CreateAccountInput{
		Stage:                            m.CreateAccount.Stage,
		PartyID:                          m.CreateAccount.PartyID,
		PublicKeyFingerprint:             m.CreateAccount.PublicKeyFingerprint,
		TopologyTransactions:             cloneMetadataBytes2D(m.CreateAccount.TopologyTransactions),
		SetupProposalPreparedTransaction: append([]byte(nil), m.CreateAccount.SetupProposalPreparedTransaction...),
		SetupProposalHash:                append([]byte(nil), m.CreateAccount.SetupProposalHash...),
		SetupProposalHashing:             m.CreateAccount.SetupProposalHashing,
		SetupProposalSubmissionID:        m.CreateAccount.SetupProposalSubmissionID,
		Signature:                        append([]byte(nil), signature...),
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
