package tx

import (
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	cantonaddress "github.com/cordialsys/crosschain/chain/canton/address"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	v2 "github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2"
	"github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive"
	v1 "github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive/transaction/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Tx for Canton holds the data needed for the external-party signing flow:
//  1. PreparedTransaction: the proto from InteractiveSubmissionService.PrepareSubmission
//  2. After SetSignatures: Serialize() marshals the ExecuteSubmissionRequest proto
type Tx struct {
	PreparedTransaction *interactive.PreparedTransaction
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

// NewTx constructs a Tx from a TxInput and transfer args, validating that the
// receiver and amount encoded in the prepared transaction match the transfer args.
//
// decimals is the chain's native asset decimal places, used to compare blockchain amounts
// against the human-readable amounts encoded in the prepared transaction.
func NewTx(input *tx_input.TxInput, args xcbuilder.TransferArgs, decimals int32) (*Tx, error) {
	_, fingerprint, err := cantonaddress.ParsePartyID(args.GetFrom())
	if err != nil {
		return nil, fmt.Errorf("failed to parse sender party ID: %w", err)
	}

	preparedTx := &input.PreparedTransaction
	if preparedTx == nil || preparedTx.GetTransaction() == nil {
		return nil, fmt.Errorf("prepared transaction is nil")
	}
	if _, err := tx_input.ComputePreparedTransactionHash(preparedTx); err != nil {
		return nil, err
	}
	if err := validateTransferArgs(preparedTx, args, decimals); err != nil {
		return nil, err
	}
	return &Tx{
		PreparedTransaction:  preparedTx,
		HashingSchemeVersion: input.HashingSchemeVersion,
		Party:                string(args.GetFrom()),
		KeyFingerprint:       fingerprint,
		SubmissionId:         input.SubmissionId,
	}, nil
}

// validateTransferArgs walks the DamlTransaction nodes to confirm the encoded receiver and
// amount match the transfer args.  Both TransferOffer (Create) and TransferPreapproval_Send
// (Exercise) flows are handled.
func validateTransferArgs(preparedTx *interactive.PreparedTransaction, args xcbuilder.TransferArgs, decimals int32) error {
	damlTx := preparedTx.GetTransaction()
	if damlTx == nil {
		return fmt.Errorf("prepared transaction contains no DamlTransaction")
	}

	wantReceiver := string(args.GetTo())
	wantAmount := args.GetAmount()

	for _, node := range damlTx.GetNodes() {
		v1Node := node.GetV1()
		if v1Node == nil {
			continue
		}

		if create := v1Node.GetCreate(); create != nil {
			if err := validateCreateNode(create, wantReceiver, wantAmount, decimals); err != nil {
				return err
			}
		}

		if exercise := v1Node.GetExercise(); exercise != nil {
			if err := validateExerciseNode(exercise, wantReceiver, wantAmount, decimals); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateCreateNode checks a TransferOffer Create node for matching receiver and amount.
func validateCreateNode(create *v1.Create, wantReceiver string, wantAmount xc.AmountBlockchain, decimals int32) error {
	tid := create.GetTemplateId()
	if tid == nil || tid.GetEntityName() != "TransferOffer" {
		return nil
	}

	arg := create.GetArgument()
	if arg == nil {
		return nil
	}
	record := arg.GetRecord()
	if record == nil {
		return nil
	}

	return validateRecordReceiverAndAmount(record, wantReceiver, wantAmount, decimals, "TransferOffer")
}

// validateExerciseNode checks a TransferPreapproval_Send Exercise node for matching receiver and amount.
func validateExerciseNode(exercise *v1.Exercise, wantReceiver string, wantAmount xc.AmountBlockchain, decimals int32) error {
	if exercise.GetChoiceId() != "TransferPreapproval_Send" {
		return nil
	}

	chosen := exercise.GetChosenValue()
	if chosen == nil {
		return nil
	}
	record := chosen.GetRecord()
	if record == nil {
		return nil
	}

	// The receiver for TransferPreapproval_Send is the contract's stakeholder (the preapproval
	// owner), not a field in the choice argument. Validate only the amount here.
	return validateAmountField(record, wantAmount, decimals, "TransferPreapproval_Send")
}

// validateRecordReceiverAndAmount checks both "receiver" party and "amount.amount" numeric fields.
func validateRecordReceiverAndAmount(record *v2.Record, wantReceiver string, wantAmount xc.AmountBlockchain, decimals int32, context string) error {
	var gotReceiver string
	var amountErr error
	amountValidated := false

	for _, field := range record.GetFields() {
		switch field.GetLabel() {
		case "receiver":
			gotReceiver = field.GetValue().GetParty()
		case "amount":
			amountErr = checkAmountField(field.GetValue(), wantAmount, decimals, context)
			amountValidated = true
		}
	}

	if gotReceiver != "" && gotReceiver != wantReceiver {
		return fmt.Errorf("%s: receiver mismatch: transaction encodes %q, args specify %q", context, gotReceiver, wantReceiver)
	}
	if amountValidated && amountErr != nil {
		return amountErr
	}

	return nil
}

// validateAmountField finds the "amount" field in a record and validates it.
func validateAmountField(record *v2.Record, wantAmount xc.AmountBlockchain, decimals int32, context string) error {
	for _, field := range record.GetFields() {
		if field.GetLabel() == "amount" {
			return checkAmountField(field.GetValue(), wantAmount, decimals, context)
		}
	}
	return nil
}

// checkAmountField validates a value that represents the transfer amount.
// The value may be a direct Numeric or a nested Record with an "amount" Numeric sub-field
// (as in the TransferOffer ExpiringAmount schema).
func checkAmountField(val *v2.Value, wantAmount xc.AmountBlockchain, decimals int32, context string) error {
	if val == nil {
		return nil
	}

	// Direct numeric (TransferPreapproval_Send choice arg)
	if numeric := val.GetNumeric(); numeric != "" {
		return compareNumericToBlockchain(numeric, wantAmount, decimals, context)
	}

	// Nested record – look for an inner "amount" numeric field (TransferOffer ExpiringAmount)
	if rec := val.GetRecord(); rec != nil {
		for _, f := range rec.GetFields() {
			if f.GetLabel() == "amount" {
				if numeric := f.GetValue().GetNumeric(); numeric != "" {
					return compareNumericToBlockchain(numeric, wantAmount, decimals, context)
				}
			}
		}
	}

	return nil
}

func compareNumericToBlockchain(numeric string, wantAmount xc.AmountBlockchain, decimals int32, context string) error {
	human, err := xc.NewAmountHumanReadableFromStr(numeric)
	if err != nil {
		return fmt.Errorf("%s: failed to parse amount %q: %w", context, numeric, err)
	}
	gotAmount := human.ToBlockchain(decimals)
	if gotAmount.Cmp(&wantAmount) != 0 {
		return fmt.Errorf("%s: amount mismatch: transaction encodes %s (%s blockchain units), args specify %s blockchain units",
			context, numeric, gotAmount.String(), wantAmount.String())
	}
	return nil
}

func (tx Tx) computedSighash() ([]byte, error) {
	if tx.PreparedTransaction == nil {
		return nil, fmt.Errorf("prepared transaction is nil")
	}
	return tx_input.ComputePreparedTransactionHash(tx.PreparedTransaction)
}

// Hash returns a hex string of the locally derived prepared transaction hash.
func (tx Tx) Hash() xc.TxHash {
	hash, err := tx.computedSighash()
	if err != nil {
		return ""
	}
	return xc.TxHash(fmt.Sprintf("%x", hash))
}

// Sighashes returns the locally derived prepared transaction hash bytes for the party to sign.
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	hash, err := tx.computedSighash()
	if err != nil {
		return nil, err
	}
	return []*xc.SignatureRequest{xc.NewSignatureRequest(hash)}, nil
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
