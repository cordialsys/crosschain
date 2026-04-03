package types

import (
	"time"

	v2 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/admin"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	"google.golang.org/protobuf/types/known/durationpb"
)

const defaultDeduplicationWindow = 300 * time.Second

func ResolveDeduplicationWindow(window time.Duration) time.Duration {
	if window <= 0 {
		return defaultDeduplicationWindow
	}
	return window
}

// func NewCommandID() string {
// 	b := make([]byte, 16)
// 	_, _ = rand.Read(b)
// 	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
// 		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
// }

func EmptyRecordValue() *v2.Value {
	return RecordValue()
}

func PartyValue(party string) *v2.Value {
	return &v2.Value{Sum: &v2.Value_Party{Party: party}}
}

func TextValue(text string) *v2.Value {
	return &v2.Value{Sum: &v2.Value_Text{Text: text}}
}

func NumericValue(numeric string) *v2.Value {
	return &v2.Value{Sum: &v2.Value_Numeric{Numeric: numeric}}
}

func ContractIDValue(contractID string) *v2.Value {
	return &v2.Value{Sum: &v2.Value_ContractId{ContractId: contractID}}
}

func RecordValue(fields ...*v2.RecordField) *v2.Value {
	return &v2.Value{
		Sum: &v2.Value_Record{
			Record: &v2.Record{Fields: fields},
		},
	}
}

func Field(label string, value *v2.Value) *v2.RecordField {
	return &v2.RecordField{Label: label, Value: value}
}

func NewPrepareRequest(commandID string, synchronizerID string, actAs []string, readAs []string, commands []*v2.Command, disclosed []*v2.DisclosedContract) *interactive.PrepareSubmissionRequest {
	return &interactive.PrepareSubmissionRequest{
		CommandId:          commandID,
		Commands:           commands,
		ActAs:              actAs,
		ReadAs:             readAs,
		SynchronizerId:     synchronizerID,
		DisclosedContracts: disclosed,
		VerboseHashing:     false,
	}
}

func NewRawSignature(signature []byte, signedBy string) *v2.Signature {
	return &v2.Signature{
		Format:               v2.SignatureFormat_SIGNATURE_FORMAT_RAW,
		Signature:            signature,
		SignedBy:             signedBy,
		SigningAlgorithmSpec: v2.SigningAlgorithmSpec_SIGNING_ALGORITHM_SPEC_ED25519,
	}
}

func NewPartySignatures(party string, sig *v2.Signature) *interactive.PartySignatures {
	return &interactive.PartySignatures{
		Signatures: []*interactive.SinglePartySignatures{
			{Party: party, Signatures: []*v2.Signature{sig}},
		},
	}
}

func NewExecuteSubmissionRequest(prepared *interactive.PreparedTransaction, party string, signature []byte, signedBy string, submissionID string, hashing interactive.HashingSchemeVersion, deduplicationWindow time.Duration) *interactive.ExecuteSubmissionRequest {
	return &interactive.ExecuteSubmissionRequest{
		PreparedTransaction: prepared,
		PartySignatures:     NewPartySignatures(party, NewRawSignature(signature, signedBy)),
		DeduplicationPeriod: &interactive.ExecuteSubmissionRequest_DeduplicationDuration{
			DeduplicationDuration: durationpb.New(ResolveDeduplicationWindow(deduplicationWindow)),
		},
		SubmissionId:         submissionID,
		HashingSchemeVersion: hashing,
	}
}

func NewExecuteSubmissionAndWaitRequest(prepared *interactive.PreparedTransaction, party string, signature []byte, signedBy string, submissionID string, hashing interactive.HashingSchemeVersion, deduplicationWindow time.Duration) *interactive.ExecuteSubmissionAndWaitRequest {
	return &interactive.ExecuteSubmissionAndWaitRequest{
		PreparedTransaction: prepared,
		PartySignatures:     NewPartySignatures(party, NewRawSignature(signature, signedBy)),
		DeduplicationPeriod: &interactive.ExecuteSubmissionAndWaitRequest_DeduplicationDuration{
			DeduplicationDuration: durationpb.New(ResolveDeduplicationWindow(deduplicationWindow)),
		},
		SubmissionId:         submissionID,
		HashingSchemeVersion: hashing,
	}
}

func NewAllocateExternalPartyRequest(synchronizerID string, topologyTxs [][]byte, signature []byte, signedBy string) *admin.AllocateExternalPartyRequest {
	onboardingTxs := make([]*admin.AllocateExternalPartyRequest_SignedTransaction, 0, len(topologyTxs))
	for _, tx := range topologyTxs {
		onboardingTxs = append(onboardingTxs, &admin.AllocateExternalPartyRequest_SignedTransaction{Transaction: tx})
	}
	return &admin.AllocateExternalPartyRequest{
		Synchronizer:           synchronizerID,
		OnboardingTransactions: onboardingTxs,
		MultiHashSignatures:    []*v2.Signature{NewRawSignature(signature, signedBy)},
	}
}
