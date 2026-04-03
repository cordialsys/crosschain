package tx_input

import (
	"fmt"
	"time"

	v2 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	v1 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive/transaction/v1"
	"google.golang.org/protobuf/proto"
)

const (
	createAccountAcceptChoiceID            = "ExternalPartySetupProposal_Accept"
	createAccountValidatorRightEntityName  = "ValidatorRight"
	createAccountTransferPreapprovalEntity = "TransferPreapproval"
	createAccountProposalEntityName        = "ExternalPartySetupProposal"
)

type CreateAccountContractInfo struct {
	LfVersion    string         `json:"lf_version,omitempty"`
	ContractID   string         `json:"contract_id,omitempty"`
	PackageName  string         `json:"package_name,omitempty"`
	TemplateID   *v2.Identifier `json:"template_id,omitempty"`
	Signatories  []string       `json:"signatories,omitempty"`
	Stakeholders []string       `json:"stakeholders,omitempty"`
}

type CreateAccountAcceptExercise struct {
	Contract      CreateAccountContractInfo `json:"contract"`
	ActingParties []string                  `json:"acting_parties,omitempty"`
}

type CreateAccountAcceptProposalContract struct {
	Contract             CreateAccountContractInfo `json:"contract"`
	CreatedAt            uint64                    `json:"created_at,omitempty"`
	CreatedEventBlob     []byte                    `json:"created_event_blob,omitempty"`
	Validator            string                    `json:"validator,omitempty"`
	User                 string                    `json:"user,omitempty"`
	DSO                  string                    `json:"dso,omitempty"`
	ProposalCreatedAt    int64                     `json:"proposal_created_at,omitempty"`
	PreapprovalExpiresAt int64                     `json:"preapproval_expires_at,omitempty"`
}

type CreateAccountAcceptValidatorRight struct {
	Contract  CreateAccountContractInfo `json:"contract"`
	DSO       string                    `json:"dso,omitempty"`
	User      string                    `json:"user,omitempty"`
	Validator string                    `json:"validator,omitempty"`
}

type CreateAccountAcceptTransferPreapproval struct {
	Contract      CreateAccountContractInfo `json:"contract"`
	DSO           string                    `json:"dso,omitempty"`
	Receiver      string                    `json:"receiver,omitempty"`
	Provider      string                    `json:"provider,omitempty"`
	ValidFrom     int64                     `json:"valid_from,omitempty"`
	LastRenewedAt int64                     `json:"last_renewed_at,omitempty"`
	ExpiresAt     int64                     `json:"expires_at,omitempty"`
}

type CreateAccountNodeSeed struct {
	NodeID int32  `json:"node_id"`
	Seed   []byte `json:"seed,omitempty"`
}

type CreateAccountAcceptInput struct {
	TransactionVersion     string                                 `json:"transaction_version,omitempty"`
	SubmitterActAs         []string                               `json:"submitter_act_as,omitempty"`
	SynchronizerID         string                                 `json:"synchronizer_id,omitempty"`
	MediatorGroup          uint32                                 `json:"mediator_group,omitempty"`
	CommandID              string                                 `json:"command_id,omitempty"`
	SubmissionID           string                                 `json:"submission_id,omitempty"`
	Hashing                interactive.HashingSchemeVersion       `json:"hashing,omitempty"`
	TransactionUUID        string                                 `json:"transaction_uuid,omitempty"`
	PreparationTime        int64                                  `json:"preparation_time,omitempty"`
	MinLedgerEffectiveTime *uint64                                `json:"min_ledger_effective_time,omitempty"`
	MaxLedgerEffectiveTime *uint64                                `json:"max_ledger_effective_time,omitempty"`
	Exercise               CreateAccountAcceptExercise            `json:"exercise"`
	ProposalInputContract  CreateAccountAcceptProposalContract    `json:"proposal_input_contract"`
	ValidatorRight         CreateAccountAcceptValidatorRight      `json:"validator_right"`
	TransferPreapproval    CreateAccountAcceptTransferPreapproval `json:"transfer_preapproval"`
	NodeSeeds              []CreateAccountNodeSeed                `json:"node_seeds,omitempty"`
}

func (input *CreateAccountAcceptInput) Clone() *CreateAccountAcceptInput {
	if input == nil {
		return nil
	}
	cloned := *input
	cloned.SubmitterActAs = append([]string(nil), input.SubmitterActAs...)
	cloned.Exercise = input.Exercise.clone()
	cloned.ProposalInputContract = input.ProposalInputContract.clone()
	cloned.ValidatorRight = input.ValidatorRight.clone()
	cloned.TransferPreapproval = input.TransferPreapproval.clone()
	cloned.NodeSeeds = cloneCreateAccountNodeSeeds(input.NodeSeeds)
	cloned.MinLedgerEffectiveTime = cloneOptionalUint64(input.MinLedgerEffectiveTime)
	cloned.MaxLedgerEffectiveTime = cloneOptionalUint64(input.MaxLedgerEffectiveTime)
	return &cloned
}

func (input *CreateAccountAcceptInput) SetUnix(unix int64) {
	if input == nil {
		return
	}
	input.PreparationTime = time.Unix(unix, 0).UTC().UnixMicro()
}

func (input *CreateAccountAcceptInput) Validate() error {
	if input == nil {
		return fmt.Errorf("create-account accept input is nil")
	}
	if input.TransactionVersion == "" {
		return fmt.Errorf("transaction version is empty")
	}
	if len(input.SubmitterActAs) == 0 {
		return fmt.Errorf("submitter act_as is empty")
	}
	if input.SynchronizerID == "" {
		return fmt.Errorf("synchronizer ID is empty")
	}
	if input.CommandID == "" {
		return fmt.Errorf("command ID is empty")
	}
	if input.TransactionUUID == "" {
		return fmt.Errorf("transaction UUID is empty")
	}
	if input.PreparationTime == 0 {
		return fmt.Errorf("preparation time is empty")
	}
	if err := validateCreateAccountExercise(input.Exercise); err != nil {
		return err
	}
	if err := validateCreateAccountProposalInputContract(input.ProposalInputContract); err != nil {
		return err
	}
	if err := validateCreateAccountValidatorRight(input.ValidatorRight); err != nil {
		return err
	}
	if err := validateCreateAccountTransferPreapproval(input.TransferPreapproval); err != nil {
		return err
	}
	if len(input.NodeSeeds) == 0 {
		return fmt.Errorf("node seeds are empty")
	}
	for _, seed := range input.NodeSeeds {
		if len(seed.Seed) == 0 {
			return fmt.Errorf("node seed for node %d is empty", seed.NodeID)
		}
	}
	return nil
}

func ParseCreateAccountAcceptInput(preparedTx *interactive.PreparedTransaction) (*CreateAccountAcceptInput, error) {
	if preparedTx == nil {
		return nil, fmt.Errorf("prepared transaction is nil")
	}
	damlTx := preparedTx.GetTransaction()
	if damlTx == nil {
		return nil, fmt.Errorf("prepared transaction contains no DamlTransaction")
	}
	if len(damlTx.GetRoots()) != 1 {
		return nil, fmt.Errorf("expected 1 root node, got %d", len(damlTx.GetRoots()))
	}
	nodes := make(map[string]*interactive.DamlTransaction_Node, len(damlTx.GetNodes()))
	for _, node := range damlTx.GetNodes() {
		nodes[node.GetNodeId()] = node
	}
	rootID := damlTx.GetRoots()[0]
	rootNode, ok := nodes[rootID]
	if !ok {
		return nil, fmt.Errorf("root node %q not found", rootID)
	}
	exercise := rootNode.GetV1().GetExercise()
	if exercise == nil {
		return nil, fmt.Errorf("root node %q is not an exercise", rootID)
	}
	if exercise.GetChoiceId() != createAccountAcceptChoiceID {
		return nil, fmt.Errorf("unexpected create-account accept choice %q", exercise.GetChoiceId())
	}

	proposalInputContract, err := parseCreateAccountProposalInputContract(preparedTx.GetMetadata())
	if err != nil {
		return nil, err
	}
	if proposalInputContract.Contract.ContractID != exercise.GetContractId() {
		return nil, fmt.Errorf("proposal contract mismatch: metadata=%q exercise=%q", proposalInputContract.Contract.ContractID, exercise.GetContractId())
	}

	validatorRight, transferPreapproval, err := parseCreateAccountAcceptChildren(exercise.GetChildren(), nodes)
	if err != nil {
		return nil, err
	}

	input := &CreateAccountAcceptInput{
		TransactionVersion: damlTx.GetVersion(),
		Exercise: CreateAccountAcceptExercise{
			Contract:      newCreateAccountContractInfoFromExercise(exercise),
			ActingParties: append([]string(nil), exercise.GetActingParties()...),
		},
		ProposalInputContract: proposalInputContract,
		ValidatorRight:        validatorRight,
		TransferPreapproval:   transferPreapproval,
		NodeSeeds:             newCreateAccountNodeSeeds(damlTx.GetNodeSeeds()),
	}

	metadata := preparedTx.GetMetadata()
	if metadata != nil {
		if submitter := metadata.GetSubmitterInfo(); submitter != nil {
			input.SubmitterActAs = append([]string(nil), submitter.GetActAs()...)
			input.CommandID = submitter.GetCommandId()
		}
		input.SynchronizerID = metadata.GetSynchronizerId()
		input.MediatorGroup = metadata.GetMediatorGroup()
		input.TransactionUUID = metadata.GetTransactionUuid()
		input.PreparationTime = int64(metadata.GetPreparationTime())
		input.MinLedgerEffectiveTime = cloneOptionalUint64(metadata.MinLedgerEffectiveTime)
		input.MaxLedgerEffectiveTime = cloneOptionalUint64(metadata.MaxLedgerEffectiveTime)
	}

	if err := input.Validate(); err != nil {
		return nil, err
	}
	return input, nil
}

func validateCreateAccountContractInfo(name string, info CreateAccountContractInfo) error {
	if info.LfVersion == "" {
		return fmt.Errorf("%s lf version is empty", name)
	}
	if info.ContractID == "" {
		return fmt.Errorf("%s contract ID is empty", name)
	}
	if info.PackageName == "" {
		return fmt.Errorf("%s package name is empty", name)
	}
	if info.TemplateID == nil {
		return fmt.Errorf("%s template ID is nil", name)
	}
	if len(info.Signatories) == 0 {
		return fmt.Errorf("%s signatories are empty", name)
	}
	if len(info.Stakeholders) == 0 {
		return fmt.Errorf("%s stakeholders are empty", name)
	}
	return nil
}

func validateCreateAccountExercise(exercise CreateAccountAcceptExercise) error {
	if err := validateCreateAccountContractInfo("accept exercise", exercise.Contract); err != nil {
		return err
	}
	if len(exercise.ActingParties) == 0 {
		return fmt.Errorf("accept exercise acting parties are empty")
	}
	return nil
}

func validateCreateAccountProposalInputContract(contract CreateAccountAcceptProposalContract) error {
	if err := validateCreateAccountContractInfo("proposal input contract", contract.Contract); err != nil {
		return err
	}
	if contract.CreatedAt == 0 {
		return fmt.Errorf("proposal input contract created_at is empty")
	}
	if len(contract.CreatedEventBlob) == 0 {
		return fmt.Errorf("proposal input contract created event blob is empty")
	}
	if contract.Validator == "" || contract.User == "" || contract.DSO == "" {
		return fmt.Errorf("proposal input contract parties are incomplete")
	}
	if contract.ProposalCreatedAt == 0 {
		return fmt.Errorf("proposal input contract createdAt is empty")
	}
	if contract.PreapprovalExpiresAt == 0 {
		return fmt.Errorf("proposal input contract preapprovalExpiresAt is empty")
	}
	return nil
}

func validateCreateAccountValidatorRight(contract CreateAccountAcceptValidatorRight) error {
	if err := validateCreateAccountContractInfo("validator right", contract.Contract); err != nil {
		return err
	}
	if contract.DSO == "" || contract.User == "" || contract.Validator == "" {
		return fmt.Errorf("validator right parties are incomplete")
	}
	return nil
}

func validateCreateAccountTransferPreapproval(contract CreateAccountAcceptTransferPreapproval) error {
	if err := validateCreateAccountContractInfo("transfer preapproval", contract.Contract); err != nil {
		return err
	}
	if contract.DSO == "" || contract.Receiver == "" || contract.Provider == "" {
		return fmt.Errorf("transfer preapproval parties are incomplete")
	}
	if contract.ValidFrom == 0 || contract.LastRenewedAt == 0 || contract.ExpiresAt == 0 {
		return fmt.Errorf("transfer preapproval timestamps are incomplete")
	}
	return nil
}

func parseCreateAccountProposalInputContract(metadata *interactive.Metadata) (CreateAccountAcceptProposalContract, error) {
	if metadata == nil {
		return CreateAccountAcceptProposalContract{}, fmt.Errorf("prepared transaction metadata is nil")
	}
	if len(metadata.GetInputContracts()) != 1 {
		return CreateAccountAcceptProposalContract{}, fmt.Errorf("expected 1 input contract, got %d", len(metadata.GetInputContracts()))
	}
	create := metadata.GetInputContracts()[0].GetV1()
	if create == nil {
		return CreateAccountAcceptProposalContract{}, fmt.Errorf("input contract is missing create payload")
	}
	record := create.GetArgument().GetRecord()
	if record == nil {
		return CreateAccountAcceptProposalContract{}, fmt.Errorf("proposal input contract argument is not a record")
	}
	proposal := CreateAccountAcceptProposalContract{
		Contract:         newCreateAccountContractInfoFromCreate(create),
		CreatedAt:        metadata.GetInputContracts()[0].GetCreatedAt(),
		CreatedEventBlob: append([]byte(nil), metadata.GetInputContracts()[0].GetEventBlob()...),
		Validator:        recordPartyField(record, "validator"),
		User:             recordPartyField(record, "user"),
		DSO:              recordPartyField(record, "dso"),
		ProposalCreatedAt: recordTimestampField(record,
			"createdAt"),
		PreapprovalExpiresAt: recordTimestampField(record, "preapprovalExpiresAt"),
	}
	if proposal.Contract.TemplateID == nil || proposal.Contract.TemplateID.GetEntityName() != createAccountProposalEntityName {
		return CreateAccountAcceptProposalContract{}, fmt.Errorf("unexpected proposal input contract template %q", proposal.Contract.TemplateID.GetEntityName())
	}
	return proposal, nil
}

func parseCreateAccountAcceptChildren(childIDs []string, nodes map[string]*interactive.DamlTransaction_Node) (CreateAccountAcceptValidatorRight, CreateAccountAcceptTransferPreapproval, error) {
	var validatorRight CreateAccountAcceptValidatorRight
	var transferPreapproval CreateAccountAcceptTransferPreapproval
	var foundValidatorRight bool
	var foundTransferPreapproval bool

	for _, childID := range childIDs {
		child, ok := nodes[childID]
		if !ok {
			return CreateAccountAcceptValidatorRight{}, CreateAccountAcceptTransferPreapproval{}, fmt.Errorf("child node %q not found", childID)
		}
		create := child.GetV1().GetCreate()
		if create == nil {
			return CreateAccountAcceptValidatorRight{}, CreateAccountAcceptTransferPreapproval{}, fmt.Errorf("child node %q is not a create", childID)
		}
		record := create.GetArgument().GetRecord()
		if record == nil {
			return CreateAccountAcceptValidatorRight{}, CreateAccountAcceptTransferPreapproval{}, fmt.Errorf("create child %q argument is not a record", childID)
		}
		entityName := create.GetTemplateId().GetEntityName()
		switch entityName {
		case createAccountValidatorRightEntityName:
			validatorRight = CreateAccountAcceptValidatorRight{
				Contract:  newCreateAccountContractInfoFromCreate(create),
				DSO:       recordPartyField(record, "dso"),
				User:      recordPartyField(record, "user"),
				Validator: recordPartyField(record, "validator"),
			}
			foundValidatorRight = true
		case createAccountTransferPreapprovalEntity:
			transferPreapproval = CreateAccountAcceptTransferPreapproval{
				Contract:      newCreateAccountContractInfoFromCreate(create),
				DSO:           recordPartyField(record, "dso"),
				Receiver:      recordPartyField(record, "receiver"),
				Provider:      recordPartyField(record, "provider"),
				ValidFrom:     recordTimestampField(record, "validFrom"),
				LastRenewedAt: recordTimestampField(record, "lastRenewedAt"),
				ExpiresAt:     recordTimestampField(record, "expiresAt"),
			}
			foundTransferPreapproval = true
		default:
			return CreateAccountAcceptValidatorRight{}, CreateAccountAcceptTransferPreapproval{}, fmt.Errorf("unexpected create-account accept child template %q", entityName)
		}
	}

	if !foundValidatorRight {
		return CreateAccountAcceptValidatorRight{}, CreateAccountAcceptTransferPreapproval{}, fmt.Errorf("validator right child node not found")
	}
	if !foundTransferPreapproval {
		return CreateAccountAcceptValidatorRight{}, CreateAccountAcceptTransferPreapproval{}, fmt.Errorf("transfer preapproval child node not found")
	}
	return validatorRight, transferPreapproval, nil
}

func newCreateAccountContractInfoFromCreate(create *v1.Create) CreateAccountContractInfo {
	if create == nil {
		return CreateAccountContractInfo{}
	}
	return CreateAccountContractInfo{
		LfVersion:    create.GetLfVersion(),
		ContractID:   create.GetContractId(),
		PackageName:  create.GetPackageName(),
		TemplateID:   cloneIdentifier(create.GetTemplateId()),
		Signatories:  append([]string(nil), create.GetSignatories()...),
		Stakeholders: append([]string(nil), create.GetStakeholders()...),
	}
}

func newCreateAccountContractInfoFromExercise(exercise *v1.Exercise) CreateAccountContractInfo {
	if exercise == nil {
		return CreateAccountContractInfo{}
	}
	return CreateAccountContractInfo{
		LfVersion:    exercise.GetLfVersion(),
		ContractID:   exercise.GetContractId(),
		PackageName:  exercise.GetPackageName(),
		TemplateID:   cloneIdentifier(exercise.GetTemplateId()),
		Signatories:  append([]string(nil), exercise.GetSignatories()...),
		Stakeholders: append([]string(nil), exercise.GetStakeholders()...),
	}
}

func recordPartyField(record *v2.Record, label string) string {
	for _, field := range record.GetFields() {
		if field.GetLabel() == label && field.GetValue() != nil {
			return field.GetValue().GetParty()
		}
	}
	return ""
}

func recordTimestampField(record *v2.Record, label string) int64 {
	for _, field := range record.GetFields() {
		if field.GetLabel() == label && field.GetValue() != nil {
			return field.GetValue().GetTimestamp()
		}
	}
	return 0
}

func cloneIdentifier(identifier *v2.Identifier) *v2.Identifier {
	if identifier == nil {
		return nil
	}
	cloned, ok := proto.Clone(identifier).(*v2.Identifier)
	if !ok {
		return nil
	}
	return cloned
}

func cloneOptionalUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func newCreateAccountNodeSeeds(seeds []*interactive.DamlTransaction_NodeSeed) []CreateAccountNodeSeed {
	if len(seeds) == 0 {
		return nil
	}
	cloned := make([]CreateAccountNodeSeed, len(seeds))
	for i, seed := range seeds {
		cloned[i] = CreateAccountNodeSeed{
			NodeID: seed.GetNodeId(),
			Seed:   append([]byte(nil), seed.GetSeed()...),
		}
	}
	return cloned
}

func cloneCreateAccountNodeSeeds(seeds []CreateAccountNodeSeed) []CreateAccountNodeSeed {
	if len(seeds) == 0 {
		return nil
	}
	cloned := make([]CreateAccountNodeSeed, len(seeds))
	copy(cloned, seeds)
	for i := range cloned {
		cloned[i].Seed = append([]byte(nil), seeds[i].Seed...)
	}
	return cloned
}

func (contract CreateAccountContractInfo) clone() CreateAccountContractInfo {
	cloned := contract
	cloned.TemplateID = cloneIdentifier(contract.TemplateID)
	cloned.Signatories = append([]string(nil), contract.Signatories...)
	cloned.Stakeholders = append([]string(nil), contract.Stakeholders...)
	return cloned
}

func (exercise CreateAccountAcceptExercise) clone() CreateAccountAcceptExercise {
	cloned := exercise
	cloned.Contract = exercise.Contract.clone()
	cloned.ActingParties = append([]string(nil), exercise.ActingParties...)
	return cloned
}

func (contract CreateAccountAcceptProposalContract) clone() CreateAccountAcceptProposalContract {
	cloned := contract
	cloned.Contract = contract.Contract.clone()
	cloned.CreatedEventBlob = append([]byte(nil), contract.CreatedEventBlob...)
	return cloned
}

func (contract CreateAccountAcceptValidatorRight) clone() CreateAccountAcceptValidatorRight {
	cloned := contract
	cloned.Contract = contract.Contract.clone()
	return cloned
}

func (contract CreateAccountAcceptTransferPreapproval) clone() CreateAccountAcceptTransferPreapproval {
	cloned := contract
	cloned.Contract = contract.Contract.clone()
	return cloned
}
