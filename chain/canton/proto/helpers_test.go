package proto

import (
	"testing"
	"time"

	v2 "github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2"
	"github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive"
	"github.com/stretchr/testify/require"
)

func TestNewPrepareRequest(t *testing.T) {
	t.Parallel()

	req := NewPrepareRequest("cmd", "sync", []string{"act"}, []string{"read"}, []*v2.Command{{}}, []*v2.DisclosedContract{{}})
	require.Equal(t, "cmd", req.GetCommandId())
	require.Equal(t, "sync", req.GetSynchronizerId())
	require.Equal(t, []string{"act"}, req.GetActAs())
	require.Equal(t, []string{"read"}, req.GetReadAs())
	require.Len(t, req.GetCommands(), 1)
	require.Len(t, req.GetDisclosedContracts(), 1)
}

func TestNewRawSignatureAndPartySignatures(t *testing.T) {
	t.Parallel()

	sig := NewRawSignature([]byte{0xaa}, "fingerprint")
	require.Equal(t, v2.SignatureFormat_SIGNATURE_FORMAT_RAW, sig.GetFormat())
	require.Equal(t, v2.SigningAlgorithmSpec_SIGNING_ALGORITHM_SPEC_ED25519, sig.GetSigningAlgorithmSpec())

	partySigs := NewPartySignatures("party", sig)
	require.Equal(t, "party", partySigs.GetSignatures()[0].GetParty())
	require.Equal(t, []byte{0xaa}, partySigs.GetSignatures()[0].GetSignatures()[0].GetSignature())
}

func TestNewExecuteRequests(t *testing.T) {
	t.Parallel()

	prepared := &interactive.PreparedTransaction{}
	hashing := interactive.HashingSchemeVersion_HASHING_SCHEME_VERSION_UNSPECIFIED

	req := NewExecuteSubmissionRequest(prepared, "party", []byte{0xaa}, "fingerprint", "sub-id", hashing)
	require.Equal(t, prepared, req.GetPreparedTransaction())
	require.Equal(t, "sub-id", req.GetSubmissionId())
	require.Equal(t, hashing, req.GetHashingSchemeVersion())
	require.Equal(t, 300*time.Second, req.GetDeduplicationDuration().AsDuration())

	waitReq := NewExecuteSubmissionAndWaitRequest(prepared, "party", []byte{0xbb}, "fingerprint", "sub-id-2", hashing)
	require.Equal(t, prepared, waitReq.GetPreparedTransaction())
	require.Equal(t, "sub-id-2", waitReq.GetSubmissionId())
	require.Equal(t, hashing, waitReq.GetHashingSchemeVersion())
	require.Equal(t, 300*time.Second, waitReq.GetDeduplicationDuration().AsDuration())
}

func TestNewAllocateExternalPartyRequest(t *testing.T) {
	t.Parallel()

	req := NewAllocateExternalPartyRequest("sync", [][]byte{{0x01}, {0x02}}, []byte{0xaa}, "fingerprint")
	require.Equal(t, "sync", req.GetSynchronizer())
	require.Len(t, req.GetOnboardingTransactions(), 2)
	require.Equal(t, []byte{0xaa}, req.GetMultiHashSignatures()[0].GetSignature())
	require.Equal(t, "fingerprint", req.GetMultiHashSignatures()[0].GetSignedBy())
}
