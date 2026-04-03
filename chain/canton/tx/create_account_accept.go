package tx

import (
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	cantonproto "github.com/cordialsys/crosschain/chain/canton/types"
	v2 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	v1 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive/transaction/v1"
)

const (
	createAccountAcceptRootNodeID                = "0"
	createAccountAcceptValidatorRightNodeID      = "1"
	createAccountAcceptTransferPreapprovalNodeID = "2"
)

func BuildCreateAccountAcceptPreparedTransaction(input *tx_input.CreateAccountAcceptInput) (*interactive.PreparedTransaction, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	rootNode, err := buildCreateAccountAcceptExerciseNode(input)
	if err != nil {
		return nil, err
	}
	validatorRightNode, err := buildCreateAccountAcceptValidatorRightNode(input.ValidatorRight)
	if err != nil {
		return nil, err
	}
	// input.TransferPreapproval.ExpiresAt += 10_000_000 // 10 seconds
	transferPreapprovalNode, err := buildCreateAccountAcceptTransferPreapprovalNode(input.TransferPreapproval)
	if err != nil {
		return nil, err
	}

	metadata, err := buildCreateAccountAcceptMetadata(input)
	if err != nil {
		return nil, err
	}

	return &interactive.PreparedTransaction{
		Transaction: &interactive.DamlTransaction{
			Version: input.TransactionVersion,
			Roots:   []string{createAccountAcceptRootNodeID},
			Nodes: []*interactive.DamlTransaction_Node{
				validatorRightNode,
				transferPreapprovalNode,
				rootNode,
			},
			NodeSeeds: buildCreateAccountNodeSeeds(input.NodeSeeds),
		},
		Metadata: metadata,
	}, nil
}

func buildCreateAccountAcceptExerciseNode(input *tx_input.CreateAccountAcceptInput) (*interactive.DamlTransaction_Node, error) {
	exerciseInfo := input.Exercise.Contract
	packageID := exerciseInfo.TemplateID.GetPackageId()
	exerciseResult := recordValue(
		identifier(packageID, exerciseInfo.TemplateID.GetModuleName(), "ExternalPartySetupProposal_AcceptResult"),
		cantonproto.Field("validatorRightCid", cantonproto.ContractIDValue(input.ValidatorRight.Contract.ContractID)),
		cantonproto.Field("transferPreapprovalCid", cantonproto.ContractIDValue(input.TransferPreapproval.Contract.ContractID)),
	)

	return &interactive.DamlTransaction_Node{
		NodeId: createAccountAcceptRootNodeID,
		VersionedNode: &interactive.DamlTransaction_Node_V1{
			V1: &v1.Node{
				NodeType: &v1.Node_Exercise{
					Exercise: &v1.Exercise{
						LfVersion:     exerciseInfo.LfVersion,
						ContractId:    exerciseInfo.ContractID,
						PackageName:   exerciseInfo.PackageName,
						TemplateId:    cloneIdentifier(exerciseInfo.TemplateID),
						Signatories:   append([]string(nil), exerciseInfo.Signatories...),
						Stakeholders:  append([]string(nil), exerciseInfo.Stakeholders...),
						ActingParties: append([]string(nil), input.Exercise.ActingParties...),
						ChoiceId:      "ExternalPartySetupProposal_Accept",
						ChosenValue:   recordValue(identifier(packageID, exerciseInfo.TemplateID.GetModuleName(), "ExternalPartySetupProposal_Accept")),
						Consuming:     true,
						Children: []string{
							createAccountAcceptValidatorRightNodeID,
							createAccountAcceptTransferPreapprovalNodeID,
						},
						ExerciseResult: exerciseResult,
					},
				},
			},
		},
	}, nil
}

func buildCreateAccountAcceptValidatorRightNode(contract tx_input.CreateAccountAcceptValidatorRight) (*interactive.DamlTransaction_Node, error) {
	info := contract.Contract
	return createAccountCreateNode(
		createAccountAcceptValidatorRightNodeID,
		info,
		recordValue(
			cloneIdentifier(info.TemplateID),
			cantonproto.Field("dso", cantonproto.PartyValue(contract.DSO)),
			cantonproto.Field("user", cantonproto.PartyValue(contract.User)),
			cantonproto.Field("validator", cantonproto.PartyValue(contract.Validator)),
		),
	), nil
}

func buildCreateAccountAcceptTransferPreapprovalNode(contract tx_input.CreateAccountAcceptTransferPreapproval) (*interactive.DamlTransaction_Node, error) {
	info := contract.Contract
	return createAccountCreateNode(
		createAccountAcceptTransferPreapprovalNodeID,
		info,
		recordValue(
			cloneIdentifier(info.TemplateID),
			cantonproto.Field("dso", cantonproto.PartyValue(contract.DSO)),
			cantonproto.Field("receiver", cantonproto.PartyValue(contract.Receiver)),
			cantonproto.Field("provider", cantonproto.PartyValue(contract.Provider)),
			cantonproto.Field("validFrom", timestampValue(contract.ValidFrom)),
			cantonproto.Field("lastRenewedAt", timestampValue(contract.LastRenewedAt)),
			cantonproto.Field("expiresAt", timestampValue(contract.ExpiresAt)), // 10 seconds
		),
	), nil
}

func buildCreateAccountAcceptMetadata(input *tx_input.CreateAccountAcceptInput) (*interactive.Metadata, error) {
	proposalInput, err := buildCreateAccountProposalInputContract(input.ProposalInputContract)
	if err != nil {
		return nil, err
	}
	return &interactive.Metadata{
		SubmitterInfo: &interactive.Metadata_SubmitterInfo{
			ActAs:     append([]string(nil), input.SubmitterActAs...),
			CommandId: input.CommandID,
		},
		SynchronizerId:         input.SynchronizerID,
		MediatorGroup:          input.MediatorGroup,
		TransactionUuid:        input.TransactionUUID,
		PreparationTime:        uint64(input.PreparationTime),
		InputContracts:         []*interactive.Metadata_InputContract{proposalInput},
		MinLedgerEffectiveTime: cloneOptionalUint64(input.MinLedgerEffectiveTime),
		MaxLedgerEffectiveTime: cloneOptionalUint64(input.MaxLedgerEffectiveTime),
	}, nil
}

func buildCreateAccountProposalInputContract(contract tx_input.CreateAccountAcceptProposalContract) (*interactive.Metadata_InputContract, error) {
	info := contract.Contract
	return &interactive.Metadata_InputContract{
		Contract: &interactive.Metadata_InputContract_V1{
			V1: &v1.Create{
				LfVersion:   info.LfVersion,
				ContractId:  info.ContractID,
				PackageName: info.PackageName,
				TemplateId:  cloneIdentifier(info.TemplateID),
				Argument: recordValue(
					cloneIdentifier(info.TemplateID),
					cantonproto.Field("validator", cantonproto.PartyValue(contract.Validator)),
					cantonproto.Field("user", cantonproto.PartyValue(contract.User)),
					cantonproto.Field("dso", cantonproto.PartyValue(contract.DSO)),
					cantonproto.Field("createdAt", timestampValue(contract.ProposalCreatedAt)),
					cantonproto.Field("preapprovalExpiresAt", timestampValue(contract.PreapprovalExpiresAt)),
				),
				Signatories:  append([]string(nil), info.Signatories...),
				Stakeholders: append([]string(nil), info.Stakeholders...),
			},
		},
		CreatedAt: contract.CreatedAt,
		EventBlob: append([]byte(nil), contract.CreatedEventBlob...),
	}, nil
}

func createAccountCreateNode(nodeID string, info tx_input.CreateAccountContractInfo, argument *v2.Value) *interactive.DamlTransaction_Node {
	return &interactive.DamlTransaction_Node{
		NodeId: nodeID,
		VersionedNode: &interactive.DamlTransaction_Node_V1{
			V1: &v1.Node{
				NodeType: &v1.Node_Create{
					Create: &v1.Create{
						LfVersion:    info.LfVersion,
						ContractId:   info.ContractID,
						PackageName:  info.PackageName,
						TemplateId:   cloneIdentifier(info.TemplateID),
						Argument:     argument,
						Signatories:  append([]string(nil), info.Signatories...),
						Stakeholders: append([]string(nil), info.Stakeholders...),
					},
				},
			},
		},
	}
}

func buildCreateAccountNodeSeeds(seeds []tx_input.CreateAccountNodeSeed) []*interactive.DamlTransaction_NodeSeed {
	if len(seeds) == 0 {
		return nil
	}
	cloned := make([]*interactive.DamlTransaction_NodeSeed, len(seeds))
	for i, seed := range seeds {
		cloned[i] = &interactive.DamlTransaction_NodeSeed{
			NodeId: seed.NodeID,
			Seed:   append([]byte(nil), seed.Seed...),
		}
	}
	return cloned
}

func recordValue(recordID *v2.Identifier, fields ...*v2.RecordField) *v2.Value {
	return &v2.Value{
		Sum: &v2.Value_Record{
			Record: &v2.Record{
				RecordId: recordID,
				Fields:   fields,
			},
		},
	}
}

func timestampValue(timestamp int64) *v2.Value {
	return &v2.Value{Sum: &v2.Value_Timestamp{Timestamp: timestamp}}
}

func identifier(packageID string, moduleName string, entityName string) *v2.Identifier {
	return &v2.Identifier{
		PackageId:  packageID,
		ModuleName: moduleName,
		EntityName: entityName,
	}
}

func cloneIdentifier(identifier *v2.Identifier) *v2.Identifier {
	if identifier == nil {
		return nil
	}
	return &v2.Identifier{
		PackageId:  identifier.GetPackageId(),
		ModuleName: identifier.GetModuleName(),
		EntityName: identifier.GetEntityName(),
	}
}

func cloneOptionalUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
