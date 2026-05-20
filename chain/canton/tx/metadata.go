package tx

import (
	"encoding/json"
	"fmt"

	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	"google.golang.org/protobuf/proto"
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

	SetupProposalPreparedTransaction *interactive.PreparedTransaction `json:"setup_proposal_prepared_transaction,omitempty"`
	SetupProposalHashing             interactive.HashingSchemeVersion `json:"setup_proposal_hashing,omitempty"`
	SetupProposalSubmissionID        string                           `json:"setup_proposal_submission_id,omitempty"`
}

type createAccountMetadataInner CreateAccountMetadata

func (m *CreateAccountMetadata) MarshalJSON() ([]byte, error) {
	var prepared json.RawMessage
	if m.SetupProposalPreparedTransaction != nil {
		bz, err := tx_input.MarshalPreparedTransactionJSON(m.SetupProposalPreparedTransaction)
		if err != nil {
			return nil, err
		}
		prepared = bz
	}
	type createAccountMetadataJSON struct {
		*createAccountMetadataInner
		SetupProposalPreparedTransaction json.RawMessage `json:"setup_proposal_prepared_transaction,omitempty"`
	}
	return json.Marshal(createAccountMetadataJSON{
		createAccountMetadataInner:       (*createAccountMetadataInner)(m),
		SetupProposalPreparedTransaction: prepared,
	})
}

func (m *CreateAccountMetadata) UnmarshalJSON(data []byte) error {
	type createAccountMetadataJSON struct {
		*createAccountMetadataInner
		SetupProposalPreparedTransaction json.RawMessage `json:"setup_proposal_prepared_transaction,omitempty"`
	}
	wire := createAccountMetadataJSON{
		createAccountMetadataInner: (*createAccountMetadataInner)(m),
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	m.SetupProposalPreparedTransaction = nil
	if len(wire.SetupProposalPreparedTransaction) > 0 && string(wire.SetupProposalPreparedTransaction) != "null" {
		prepared, err := tx_input.UnmarshalPreparedTransactionJSON(wire.SetupProposalPreparedTransaction)
		if err != nil {
			return err
		}
		m.SetupProposalPreparedTransaction = prepared
	}
	return nil
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
			SetupProposalPreparedTransaction: clonePreparedTransaction(input.SetupProposalPreparedTransaction),
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
		SetupProposalPreparedTransaction: clonePreparedTransaction(m.CreateAccount.SetupProposalPreparedTransaction),
		SetupProposalHashing:             m.CreateAccount.SetupProposalHashing,
		SetupProposalSubmissionID:        m.CreateAccount.SetupProposalSubmissionID,
		Signature:                        append([]byte(nil), signature...),
	}
	if err := input.VerifySignaturePayloads(); err != nil {
		return nil, fmt.Errorf("invalid create-account metadata payload: %w", err)
	}
	return input, nil
}

func clonePreparedTransaction(prepared *interactive.PreparedTransaction) *interactive.PreparedTransaction {
	if prepared == nil {
		return nil
	}
	return proto.Clone(prepared).(*interactive.PreparedTransaction)
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
