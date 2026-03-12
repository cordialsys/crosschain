package tx

import (
	"fmt"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	cantonaddress "github.com/cordialsys/crosschain/chain/canton/address"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	cantonproto "github.com/cordialsys/crosschain/chain/canton/types"
	v2 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	v1 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive/transaction/v1"
	cantonclientconfig "github.com/cordialsys/crosschain/client/canton"
	"github.com/cordialsys/crosschain/pkg/safe_map"
	"google.golang.org/protobuf/proto"
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
	// DeduplicationWindow controls how long Canton deduplicates this submission ID.
	DeduplicationWindow time.Duration
	// LedgerEnd captured before submission, used as the lower-bound recovery cursor.
	LedgerEnd int64
	// Populated after SetSignatures is called
	signature []byte
}

var _ xc.Tx = &Tx{}
var _ xc.TxWithMetadata = &Tx{}

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

	preparedTx := input.PreparedTransaction
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
		DeduplicationWindow:  input.DeduplicationWindow,
		LedgerEnd:            input.LedgerEnd,
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
	var wantInstrumentAdmin string
	var wantInstrumentID string
	if contract, ok := args.GetContract(); ok {
		wantInstrumentAdmin, wantInstrumentID, _ = cantonclientconfig.TokenRegistryKey(contract).Parts()
	}

	type validatedTransferNode struct {
		nodeID string
		amount xc.AmountBlockchain
	}

	parentByChildID := safe_map.New[string]()
	validatedTransfers := []validatedTransferNode{}
	validatedTransferIDs := safe_map.New[bool]()
	for _, node := range damlTx.GetNodes() {
		v1Node := node.GetV1()
		if v1Node == nil {
			continue
		}
		nodeID := node.GetNodeId()

		if create := v1Node.GetCreate(); create != nil {
			amount, ok, err := validateCreateNode(create, wantReceiver, decimals, wantInstrumentAdmin, wantInstrumentID)
			if err != nil {
				return err
			}
			if ok {
				validatedTransfers = append(validatedTransfers, validatedTransferNode{nodeID: nodeID, amount: amount})
				validatedTransferIDs.Set(nodeID, true)
			}
		}

		if exercise := v1Node.GetExercise(); exercise != nil {
			for _, childID := range exercise.GetChildren() {
				parentByChildID.Set(childID, nodeID)
			}

			amount, ok, err := validateExerciseNode(exercise, wantReceiver, decimals, wantInstrumentAdmin, wantInstrumentID)
			if err != nil {
				return err
			}
			if ok {
				validatedTransfers = append(validatedTransfers, validatedTransferNode{nodeID: nodeID, amount: amount})
				validatedTransferIDs.Set(nodeID, true)
			}
		}
	}

	if len(validatedTransfers) == 0 {
		return fmt.Errorf("prepared transaction contains no recognized Canton transfer node")
	}

	totalAmount := xc.NewAmountBlockchainFromUint64(0)
	for _, transfer := range validatedTransfers {
		if hasRecognizedTransferAncestor(transfer.nodeID, parentByChildID, validatedTransferIDs) {
			continue
		}
		totalAmount = (&totalAmount).Add(&transfer.amount)
		if (&totalAmount).Cmp(&wantAmount) > 0 {
			return fmt.Errorf("prepared transaction transfer amount exceeds args: transaction encodes at least %s blockchain units, args specify %s blockchain units", totalAmount.String(), wantAmount.String())
		}
	}

	return nil
}

func hasRecognizedTransferAncestor(nodeID string, parentByChildID *safe_map.Map[string], validatedTransferIDs *safe_map.Map[bool]) bool {
	seen := safe_map.New[bool]()
	for {
		parentID, ok := parentByChildID.Get(nodeID)
		if !ok || parentID == "" || seen.Has(parentID) {
			return false
		}
		if validatedTransferIDs.Has(parentID) {
			return true
		}
		seen.Set(parentID, true)
		nodeID = parentID
	}
}

// validateCreateNode checks a TransferOffer Create node for matching receiver and amount.
func validateCreateNode(create *v1.Create, wantReceiver string, decimals int32, wantInstrumentAdmin string, wantInstrumentID string) (xc.AmountBlockchain, bool, error) {
	tid := create.GetTemplateId()
	if tid == nil || tid.GetEntityName() != "TransferOffer" {
		return xc.NewAmountBlockchainFromUint64(0), false, nil
	}

	arg := create.GetArgument()
	if arg == nil {
		return xc.NewAmountBlockchainFromUint64(0), true, fmt.Errorf("TransferOffer: missing create argument")
	}
	record := arg.GetRecord()
	if record == nil {
		return xc.NewAmountBlockchainFromUint64(0), true, fmt.Errorf("TransferOffer: create argument is not a record")
	}

	// When tested it had a string like "Utility.Registry.App.V0.Model.Transfer"
	if strings.HasSuffix(tid.GetModuleName(), ".Model.Transfer") {
		amount, err := validateTokenTransferRecord(record, wantReceiver, decimals, wantInstrumentAdmin, wantInstrumentID, "TransferOffer")
		return amount, true, err
	}

	amount, err := validateRecordReceiverAndAmount(record, wantReceiver, decimals, "TransferOffer")
	return amount, true, err
}

// validateExerciseNode checks a TransferPreapproval_Send Exercise node for matching receiver and amount.
func validateExerciseNode(exercise *v1.Exercise, wantReceiver string, decimals int32, wantInstrumentAdmin string, wantInstrumentID string) (xc.AmountBlockchain, bool, error) {
	switch exercise.GetChoiceId() {
	case "TransferPreapproval_Send":
		if !containsString(exercise.GetStakeholders(), wantReceiver) {
			return xc.NewAmountBlockchainFromUint64(0), true, fmt.Errorf("TransferPreapproval_Send: expected receiver %q is not a stakeholder on the exercised preapproval contract", wantReceiver)
		}
		chosen := exercise.GetChosenValue()
		if chosen == nil {
			return xc.NewAmountBlockchainFromUint64(0), true, fmt.Errorf("TransferPreapproval_Send: missing chosen value")
		}
		record := chosen.GetRecord()
		if record == nil {
			return xc.NewAmountBlockchainFromUint64(0), true, fmt.Errorf("TransferPreapproval_Send: chosen value is not a record")
		}
		// The receiver for TransferPreapproval_Send is the contract stakeholder, not a
		// choice argument field.
		amount, err := validateAmountField(record, decimals, "TransferPreapproval_Send")
		return amount, true, err
	case "TransferFactory_Transfer":
		chosen := exercise.GetChosenValue()
		if chosen == nil {
			return xc.NewAmountBlockchainFromUint64(0), true, fmt.Errorf("TransferFactory_Transfer: missing chosen value")
		}
		record := chosen.GetRecord()
		if record == nil {
			return xc.NewAmountBlockchainFromUint64(0), true, fmt.Errorf("TransferFactory_Transfer: chosen value is not a record")
		}
		amount, err := validateTokenTransferRecord(record, wantReceiver, decimals, wantInstrumentAdmin, wantInstrumentID, "TransferFactory_Transfer")
		return amount, true, err
	default:
		return xc.NewAmountBlockchainFromUint64(0), false, nil
	}
}

func validateTokenTransferRecord(record *v2.Record, wantReceiver string, decimals int32, wantInstrumentAdmin string, wantInstrumentID string, context string) (xc.AmountBlockchain, error) {
	if _, ok := getRecordFieldValue(record, "receiver"); ok {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: contains unexpected root receiver", context)
	}

	transferValue, ok := getRecordFieldValue(record, "transfer")
	if !ok || transferValue.GetRecord() == nil {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: missing transfer record", context)
	}
	transferRecord := transferValue.GetRecord()
	amount, err := validateRecordReceiverAndAmount(transferRecord, wantReceiver, decimals, context)
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), err
	}
	if wantInstrumentAdmin == "" && wantInstrumentID == "" {
		return amount, nil
	}
	instrumentValue, ok := getRecordFieldValue(transferRecord, "instrumentId")
	if !ok || instrumentValue.GetRecord() == nil {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: missing instrumentId record", context)
	}
	gotAdminValue, _ := getRecordFieldValue(instrumentValue.GetRecord(), "admin")
	gotIDValue, _ := getRecordFieldValue(instrumentValue.GetRecord(), "id")
	gotAdmin := ""
	gotID := ""
	if gotAdminValue != nil {
		gotAdmin = gotAdminValue.GetParty()
	}
	if gotIDValue != nil {
		gotID = gotIDValue.GetText()
	}
	if gotAdmin == "" {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: missing instrument admin", context)
	}
	if gotID == "" {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: missing instrument id", context)
	}
	if gotAdmin != "" && gotAdmin != wantInstrumentAdmin {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: instrument admin mismatch: transaction encodes %q, args specify %q", context, gotAdmin, wantInstrumentAdmin)
	}
	if gotID != "" && gotID != wantInstrumentID {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: instrument id mismatch: transaction encodes %q, args specify %q", context, gotID, wantInstrumentID)
	}
	return amount, nil
}

func getRecordFieldValue(record *v2.Record, label string) (*v2.Value, bool) {
	if record == nil {
		return nil, false
	}
	for _, field := range record.GetFields() {
		if field.GetLabel() == label {
			return field.GetValue(), true
		}
	}
	return nil, false
}

// validateRecordReceiverAndAmount checks both "receiver" party and amount fields.
func validateRecordReceiverAndAmount(record *v2.Record, wantReceiver string, decimals int32, context string) (xc.AmountBlockchain, error) {
	receiverValue, ok := getRecordFieldValue(record, "receiver")
	if !ok {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: missing receiver", context)
	}
	gotReceiver := receiverValue.GetParty()
	if gotReceiver == "" {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: receiver is not a party", context)
	}
	if gotReceiver != wantReceiver {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: receiver mismatch: transaction encodes %q, args specify %q", context, gotReceiver, wantReceiver)
	}

	amountValue, ok := getRecordFieldValue(record, "amount")
	if !ok {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: missing amount", context)
	}
	return checkAmountField(amountValue, decimals, context)
}

// validateAmountField finds the "amount" field in a record and validates it.
func validateAmountField(record *v2.Record, decimals int32, context string) (xc.AmountBlockchain, error) {
	for _, field := range record.GetFields() {
		if field.GetLabel() == "amount" {
			return checkAmountField(field.GetValue(), decimals, context)
		}
	}
	return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: missing amount", context)
}

// checkAmountField validates a value that represents the transfer amount.
// The value may be a direct Numeric or a nested Record with an "amount" Numeric sub-field
// (as in the TransferOffer ExpiringAmount schema).
func checkAmountField(val *v2.Value, decimals int32, context string) (xc.AmountBlockchain, error) {
	if val == nil {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: amount is nil", context)
	}

	// Direct numeric (TransferPreapproval_Send choice arg)
	if numeric := val.GetNumeric(); numeric != "" {
		return parseNumericToBlockchain(numeric, decimals, context)
	}

	// Nested record – look for an inner "amount" numeric field (TransferOffer ExpiringAmount)
	if rec := val.GetRecord(); rec != nil {
		for _, f := range rec.GetFields() {
			if f.GetLabel() == "amount" {
				if numeric := f.GetValue().GetNumeric(); numeric != "" {
					return parseNumericToBlockchain(numeric, decimals, context)
				}
			}
		}
	}

	return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: missing numeric amount", context)
}

func parseNumericToBlockchain(numeric string, decimals int32, context string) (xc.AmountBlockchain, error) {
	human, err := xc.NewAmountHumanReadableFromStr(numeric)
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("%s: failed to parse amount %q: %w", context, numeric, err)
	}
	return human.ToBlockchain(decimals), nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (tx Tx) computedSighash() ([]byte, error) {
	if tx.PreparedTransaction == nil {
		return nil, fmt.Errorf("prepared transaction is nil")
	}
	return tx_input.ComputePreparedTransactionHash(tx.PreparedTransaction)
}

// Hash returns a recovery token in the form "<ledger_end>-<submission_id>".
func (tx Tx) Hash() xc.TxHash {
	if tx.SubmissionId == "" {
		return ""
	}
	return xc.TxHash(fmt.Sprintf("%d-%s", tx.LedgerEnd, tx.SubmissionId))
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

// Serialize marshals the ExecuteSubmissionAndWaitRequest proto so SubmitTx can send it over gRPC.
func (tx Tx) Serialize() ([]byte, error) {
	if len(tx.signature) == 0 {
		return nil, fmt.Errorf("transaction is not signed")
	}
	if tx.PreparedTransaction == nil {
		return nil, fmt.Errorf("prepared transaction is nil")
	}

	req := cantonproto.NewExecuteSubmissionAndWaitRequest(tx.PreparedTransaction, tx.Party, tx.signature, tx.KeyFingerprint, tx.SubmissionId, tx.HashingSchemeVersion, tx.DeduplicationWindow)

	data, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Canton execute-and-wait request: %w", err)
	}
	return data, nil
}

func (tx Tx) GetMetadata() ([]byte, bool, error) {
	metadata := NewTransferMetadata()
	bz, err := metadata.Bytes()
	if err != nil {
		return nil, false, err
	}
	return bz, true, nil
}
