package tx_input

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive"
	"google.golang.org/protobuf/proto"
)

const (
	CreateAccountStageAllocate = "allocate_external_party"
	CreateAccountStageAccept   = "accept_external_party_setup_proposal"
)

// Signature index constants callers use to address specific payloads.
const (
	SigIdxCreateAccount = 0
)

// CreateAccountInput holds the next Canton account-registration step that still
// requires an explicit external signature.
type CreateAccountInput struct {
	Stage string `json:"stage"`

	Description string `json:"description,omitempty"`

	PartyID              string   `json:"party_id"`
	PublicKeyFingerprint string   `json:"public_key_fingerprint,omitempty"`
	TopologyMultiHash    []byte   `json:"topology_multi_hash,omitempty"`
	TopologyTransactions [][]byte `json:"topology_transactions,omitempty"`

	SetupProposalPreparedTransaction []byte                           `json:"setup_proposal_prepared_transaction,omitempty"`
	SetupProposalHash                []byte                           `json:"setup_proposal_hash,omitempty"`
	SetupProposalHashing             interactive.HashingSchemeVersion `json:"setup_proposal_hashing,omitempty"`
	SetupProposalSubmissionID        string                           `json:"setup_proposal_submission_id,omitempty"`

	Signature []byte `json:"signature,omitempty"`
}

var _ xclient.CreateAccountInput = &CreateAccountInput{}

func ParseCreateAccountInput(data []byte) (*CreateAccountInput, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("create-account input is too short")
	}

	metadataLen := binary.BigEndian.Uint64(data[:8])
	if metadataLen > uint64(len(data)-8) {
		return nil, fmt.Errorf("invalid create-account metadata length %d", metadataLen)
	}

	var input CreateAccountInput
	metadata := data[8 : 8+metadataLen]
	if err := json.Unmarshal(metadata, &input); err != nil {
		return nil, err
	}
	if input.Stage == "" {
		return nil, fmt.Errorf("missing create-account stage")
	}
	input.Signature = append([]byte(nil), data[8+metadataLen:]...)
	return &input, nil
}

func (i *CreateAccountInput) Serialize() ([]byte, error) {
	metadata, err := i.metadataBytes()
	if err != nil {
		return nil, err
	}

	payload := make([]byte, 8+len(metadata)+len(i.Signature))
	binary.BigEndian.PutUint64(payload[:8], uint64(len(metadata)))
	copy(payload[8:], metadata)
	copy(payload[8+len(metadata):], i.Signature)
	return payload, nil
}

// Sighashes returns the payload for the current pending step.
func (i *CreateAccountInput) Sighashes() ([]*xc.SignatureRequest, error) {
	switch i.Stage {
	case CreateAccountStageAllocate:
		if len(i.TopologyMultiHash) == 0 {
			return nil, fmt.Errorf("topology multi-hash is empty")
		}
		return []*xc.SignatureRequest{xc.NewSignatureRequest(i.TopologyMultiHash)}, nil
	case CreateAccountStageAccept:
		if len(i.SetupProposalHash) == 0 {
			return nil, fmt.Errorf("setup proposal hash is empty")
		}
		return []*xc.SignatureRequest{xc.NewSignatureRequest(i.SetupProposalHash)}, nil
	default:
		return nil, fmt.Errorf("unsupported create-account stage %q", i.Stage)
	}
}

func (i *CreateAccountInput) SetSignatures(sigs ...*xc.SignatureResponse) error {
	if len(sigs) != 1 {
		return fmt.Errorf("expected 1 signature, got %d", len(sigs))
	}
	i.Signature = sigs[SigIdxCreateAccount].Signature
	return nil
}

func (i *CreateAccountInput) VerifySignaturePayloads() error {
	if i.PartyID == "" {
		return fmt.Errorf("party ID is empty")
	}
	switch i.Stage {
	case CreateAccountStageAllocate:
		if len(i.TopologyMultiHash) == 0 {
			return fmt.Errorf("topology multi-hash is empty")
		}
		if len(i.TopologyTransactions) == 0 {
			return fmt.Errorf("topology transactions are empty")
		}
	case CreateAccountStageAccept:
		if len(i.SetupProposalPreparedTransaction) == 0 {
			return fmt.Errorf("setup proposal prepared transaction is empty")
		}
		if len(i.SetupProposalHash) == 0 {
			return fmt.Errorf("setup proposal hash is empty")
		}
		var prepared interactive.PreparedTransaction
		if err := proto.Unmarshal(i.SetupProposalPreparedTransaction, &prepared); err != nil {
			return fmt.Errorf("failed to unmarshal setup proposal prepared transaction: %w", err)
		}
	default:
		return fmt.Errorf("unsupported create-account stage %q", i.Stage)
	}
	return nil
}

func (i *CreateAccountInput) metadataBytes() ([]byte, error) {
	metadata := *i
	metadata.Signature = nil
	return json.Marshal(metadata)
}
