package tx

import (
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	v2 "github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2"
	"github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Tx for Canton holds the data needed for the external-party signing flow:
//  1. PreparedTransaction: the proto from InteractiveSubmissionService.PrepareSubmission
//  2. PreparedTransactionHash: raw bytes to sign (Ed25519)
//  3. After SetSignatures: Serialize() marshals the ExecuteSubmissionRequest proto
type Tx struct {
	PreparedTransaction *interactive.PreparedTransaction
	// Raw hash bytes of the prepared transaction (what the party signs)
	PreparedTransactionHash []byte
	// Hashing scheme version returned by prepare endpoint
	HashingSchemeVersion interactive.HashingSchemeVersion
	// Party (Canton party ID / address) that is authorizing this transaction
	Party string
	// Fingerprint of the signing key (the portion after "12" in the party's fingerprint)
	KeyFingerprint string
	// SubmissionId for deduplication
	SubmissionId string
	// Populated after SetSignatures is called
	signature []byte
}

var _ xc.Tx = &Tx{}

// Hash returns a hex string of the prepared transaction hash
func (tx Tx) Hash() xc.TxHash {
	return xc.TxHash(fmt.Sprintf("%x", tx.PreparedTransactionHash))
}

// Sighashes returns the raw prepared transaction hash bytes for the party to sign
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	if len(tx.PreparedTransactionHash) == 0 {
		return nil, fmt.Errorf("prepared transaction hash is empty")
	}
	return []*xc.SignatureRequest{xc.NewSignatureRequest(tx.PreparedTransactionHash)}, nil
}

// SetSignatures stores the Ed25519 signature from the external party
func (tx *Tx) SetSignatures(sigs ...*xc.SignatureResponse) error {
	if len(sigs) != 1 {
		return fmt.Errorf("expected exactly 1 signature, got %d", len(sigs))
	}
	tx.signature = sigs[0].Signature
	return nil
}

// Serialize marshals the ExecuteSubmissionRequest proto so SubmitTx can send it over gRPC
func (tx Tx) Serialize() ([]byte, error) {
	if len(tx.signature) == 0 {
		return nil, fmt.Errorf("transaction is not signed")
	}
	if tx.PreparedTransaction == nil {
		return nil, fmt.Errorf("prepared transaction is nil")
	}

	req := &interactive.ExecuteSubmissionRequest{
		PreparedTransaction: tx.PreparedTransaction,
		PartySignatures: &interactive.PartySignatures{
			Signatures: []*interactive.SinglePartySignatures{
				{
					Party: tx.Party,
					Signatures: []*v2.Signature{
						{
							Format:               v2.SignatureFormat_SIGNATURE_FORMAT_RAW,
							Signature:            tx.signature,
							SignedBy:             tx.KeyFingerprint,
							SigningAlgorithmSpec: v2.SigningAlgorithmSpec_SIGNING_ALGORITHM_SPEC_ED25519,
						},
					},
				},
			},
		},
		DeduplicationPeriod: &interactive.ExecuteSubmissionRequest_DeduplicationDuration{
			// TODO: Move to input
			DeduplicationDuration: durationpb.New(300 * time.Second),
		},
		SubmissionId:         tx.SubmissionId,
		HashingSchemeVersion: tx.HashingSchemeVersion,
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Canton execute request: %w", err)
	}
	return data, nil
}
