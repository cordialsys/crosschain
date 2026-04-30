package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	xcbuilder "github.com/cordialsys/crosschain/builder"
	cantonproto "github.com/cordialsys/crosschain/chain/canton/types"
	v2 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2"
)

const (
	tokenTransferModule = "Splice.Api.Token.TransferInstructionV1"
	tokenTransferEntity = "TransferFactory"
)

func extractLedgerTokenFactoryConfig(created *v2.CreatedEvent) (admin string, instrumentID string, ok bool) {
	if created == nil || created.GetCreateArguments() == nil {
		return "", "", false
	}
	if _, ok := extractTransferFactoryAdmin(created); !ok {
		return "", "", false
	}
	args := created.GetCreateArguments()
	if ownerValue, hasOwner := getRecordFieldValue(args, "owner"); hasOwner && ownerValue != nil {
		// Holdings commonly expose owner/admin/symbol too; exclude them from factory discovery.
		return "", "", false
	}
	if amountValue, hasAmount := getRecordFieldValue(args, "amount"); hasAmount && amountValue != nil {
		return "", "", false
	}
	adminValue, hasAdmin := getRecordFieldValue(args, "admin")
	if hasAdmin && adminValue.GetParty() != "" {
		if symbolValue, hasSymbol := getRecordFieldValue(args, "symbol"); hasSymbol && symbolValue.GetText() != "" {
			return adminValue.GetParty(), symbolValue.GetText(), true
		}
		if instrumentValue, hasInstrument := getRecordFieldValue(args, "instrumentId"); hasInstrument && instrumentValue.GetRecord() != nil {
			instrumentAdmin, hasInstrumentAdmin := getRecordFieldValue(instrumentValue.GetRecord(), "admin")
			instrumentIDValue, hasInstrumentID := getRecordFieldValue(instrumentValue.GetRecord(), "id")
			if hasInstrumentAdmin && hasInstrumentID && instrumentAdmin.GetParty() == adminValue.GetParty() && instrumentIDValue.GetText() != "" {
				return adminValue.GetParty(), instrumentIDValue.GetText(), true
			}
		}
	}
	return "", "", false
}

func extractTransferFactoryAdmin(created *v2.CreatedEvent) (string, bool) {
	if created == nil {
		return "", false
	}
	for _, view := range created.GetInterfaceViews() {
		interfaceID := view.GetInterfaceId()
		if interfaceID == nil || interfaceID.GetModuleName() != tokenTransferModule || interfaceID.GetEntityName() != tokenTransferEntity {
			continue
		}
		adminValue, ok := getRecordFieldValue(view.GetViewValue(), "admin")
		if !ok || adminValue.GetParty() == "" {
			return "", false
		}
		return adminValue.GetParty(), true
	}
	return "", false
}

func resolveLedgerTokenTransferFactoryContract(adminContracts []*v2.ActiveContract, instrumentAdmin string, instrumentID string) (*v2.CreatedEvent, error) {
	for _, contract := range adminContracts {
		created := contract.GetCreatedEvent()
		admin, resolvedInstrumentID, ok := extractLedgerTokenFactoryConfig(created)
		if !ok {
			continue
		}
		if admin == instrumentAdmin && resolvedInstrumentID == instrumentID {
			return created, nil
		}
	}
	return nil, fmt.Errorf("no visible token transfer factory found on admin ledger view for %s#%s", instrumentAdmin, instrumentID)
}

func transferAmountNumeric(args xcbuilder.TransferArgs, decimals int32) string {
	amount := args.GetAmount()
	return amount.ToHuman(decimals).String()
}

func buildTransferOfferCreateCommand(args xcbuilder.TransferArgs, amuletRules AmuletRules, walletPackageID string, commandID string, decimals int32) *v2.Command {
	amountNumeric := transferAmountNumeric(args, decimals)
	return &v2.Command{
		Command: &v2.Command_Create{
			Create: &v2.CreateCommand{
				TemplateId: &v2.Identifier{
					PackageId:  walletPackageID,
					ModuleName: "Splice.Wallet.TransferOffer",
					EntityName: "TransferOffer",
				},
				CreateArguments: &v2.Record{
					Fields: []*v2.RecordField{
						cantonproto.Field("sender", cantonproto.PartyValue(string(args.GetFrom()))),
						cantonproto.Field("receiver", cantonproto.PartyValue(string(args.GetTo()))),
						cantonproto.Field("dso", cantonproto.PartyValue(amuletRules.AmuletRulesUpdate.Contract.Payload.DSO)),
						cantonproto.Field("amount", cantonproto.RecordValue(
							cantonproto.Field("amount", cantonproto.NumericValue(amountNumeric)),
							cantonproto.Field("unit", &v2.Value{
								Sum: &v2.Value_Enum{Enum: &v2.Enum{Constructor: "AmuletUnit"}},
							}),
						)),
						cantonproto.Field("description", cantonproto.TextValue("")),
						cantonproto.Field("expiresAt", &v2.Value{Sum: &v2.Value_Timestamp{Timestamp: time.Now().UTC().Add(24 * time.Hour).UnixMicro()}}),
						cantonproto.Field("trackingId", cantonproto.TextValue(commandID)),
					},
				},
			},
		},
	}
}

func buildExternalPartySetupProposalAcceptCommand(templateID *v2.Identifier, contractID string) *v2.Command {
	return &v2.Command{
		Command: &v2.Command_Exercise{
			Exercise: &v2.ExerciseCommand{
				TemplateId:     templateID,
				ContractId:     contractID,
				Choice:         "ExternalPartySetupProposal_Accept",
				ChoiceArgument: cantonproto.EmptyRecordValue(),
			},
		},
	}
}

func buildWalletTransferOfferAcceptCommand(templateID *v2.Identifier, contractID string) *v2.Command {
	return &v2.Command{
		Command: &v2.Command_Exercise{
			Exercise: &v2.ExerciseCommand{
				TemplateId:     templateID,
				ContractId:     contractID,
				Choice:         "TransferOffer_Accept",
				ChoiceArgument: cantonproto.EmptyRecordValue(),
			},
		},
	}
}

func buildTransferPreapprovalExerciseCommand(
	args xcbuilder.TransferArgs,
	amuletRules AmuletRules,
	openMiningRound *RoundEntry,
	issuingMiningRound *RoundEntry,
	senderContracts []*v2.ActiveContract,
	recipientContracts []*v2.ActiveContract,
	decimals int32,
) (*v2.Command, []*v2.DisclosedContract, error) {
	var preapprovalContractID string
	var preapprovalTemplateID *v2.Identifier
	for _, c := range recipientContracts {
		event := c.GetCreatedEvent()
		if event == nil {
			continue
		}
		tid := event.GetTemplateId()
		if tid != nil && isPreapprovalTemplate(tid) {
			preapprovalContractID = event.GetContractId()
			preapprovalTemplateID = tid
			break
		}
	}
	if preapprovalContractID == "" {
		return nil, nil, fmt.Errorf("no TransferPreapproval contract found for recipient %s", args.GetTo())
	}

	transferInputs := make([]*v2.Value, 0)
	disclosedContracts := make([]*v2.DisclosedContract, 0)

	for _, ac := range recipientContracts {
		event := ac.GetCreatedEvent()
		if event == nil {
			continue
		}
		if tid := event.GetTemplateId(); tid != nil && isPreapprovalTemplate(tid) {
			disclosedContracts = append(disclosedContracts, &v2.DisclosedContract{
				TemplateId:       tid,
				ContractId:       event.GetContractId(),
				CreatedEventBlob: event.GetCreatedEventBlob(),
			})
			break
		}
	}

	for _, ac := range senderContracts {
		event := ac.GetCreatedEvent()
		if event == nil || event.GetTemplateId().GetEntityName() != "Amulet" {
			continue
		}
		transferInputs = append(transferInputs, &v2.Value{
			Sum: &v2.Value_Variant{
				Variant: &v2.Variant{
					Constructor: "InputAmulet",
					Value:       cantonproto.ContractIDValue(event.GetContractId()),
				},
			},
		})
		disclosedContracts = append(disclosedContracts, &v2.DisclosedContract{
			TemplateId:       event.GetTemplateId(),
			ContractId:       event.GetContractId(),
			CreatedEventBlob: event.GetCreatedEventBlob(),
		})
	}

	amuletRulesID := amuletRules.AmuletRulesUpdate.Contract.ContractID
	openMiningRoundID := openMiningRound.Contract.ContractID

	amuletRulesTemplateParts := strings.SplitN(amuletRules.AmuletRulesUpdate.Contract.TemplateID, ":", 3)
	if len(amuletRulesTemplateParts) == 3 {
		disclosedContracts = append(disclosedContracts, &v2.DisclosedContract{
			TemplateId: &v2.Identifier{
				PackageId:  amuletRulesTemplateParts[0],
				ModuleName: amuletRulesTemplateParts[1],
				EntityName: amuletRulesTemplateParts[2],
			},
			ContractId:       amuletRulesID,
			CreatedEventBlob: amuletRules.AmuletRulesUpdate.Contract.CreatedEventBlob,
		})
	}

	openParts := strings.Split(openMiningRound.Contract.TemplateID, ":")
	if len(openParts) == 3 {
		disclosedContracts = append(disclosedContracts, &v2.DisclosedContract{
			TemplateId: &v2.Identifier{
				PackageId:  openParts[0],
				ModuleName: openParts[1],
				EntityName: openParts[2],
			},
			ContractId:       openMiningRoundID,
			CreatedEventBlob: openMiningRound.Contract.CreatedEventBlob,
		})
	}

	rn, err := strconv.ParseInt(issuingMiningRound.Contract.Payload.Round.Number, 10, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse issuing mining round number: %w", err)
	}
	issuingMiningRounds := &v2.Value{
		Sum: &v2.Value_GenMap{
			GenMap: &v2.GenMap{
				Entries: []*v2.GenMap_Entry{
					{
						Key: cantonproto.RecordValue(
							cantonproto.Field("number", &v2.Value{Sum: &v2.Value_Int64{Int64: rn}}),
						),
						Value: cantonproto.ContractIDValue(issuingMiningRound.Contract.ContractID),
					},
				},
			},
		},
	}

	issuingParts := strings.Split(issuingMiningRound.Contract.TemplateID, ":")
	if len(issuingParts) == 3 {
		disclosedContracts = append(disclosedContracts, &v2.DisclosedContract{
			TemplateId: &v2.Identifier{
				PackageId:  issuingParts[0],
				ModuleName: issuingParts[1],
				EntityName: issuingParts[2],
			},
			ContractId:       issuingMiningRound.Contract.ContractID,
			CreatedEventBlob: issuingMiningRound.Contract.CreatedEventBlob,
		})
	}

	amountNumeric := transferAmountNumeric(args, decimals)
	cmd := &v2.Command{
		Command: &v2.Command_Exercise{
			Exercise: &v2.ExerciseCommand{
				TemplateId: preapprovalTemplateID,
				ContractId: preapprovalContractID,
				Choice:     "TransferPreapproval_Send",
				ChoiceArgument: cantonproto.RecordValue(
					cantonproto.Field("sender", cantonproto.PartyValue(string(args.GetFrom()))),
					cantonproto.Field("inputs", &v2.Value{
						Sum: &v2.Value_List{List: &v2.List{Elements: transferInputs}},
					}),
					cantonproto.Field("amount", cantonproto.NumericValue(amountNumeric)),
					cantonproto.Field("context", cantonproto.RecordValue(
						cantonproto.Field("amuletRules", cantonproto.ContractIDValue(amuletRulesID)),
						cantonproto.Field("context", cantonproto.RecordValue(
							cantonproto.Field("openMiningRound", cantonproto.ContractIDValue(openMiningRoundID)),
							cantonproto.Field("issuingMiningRounds", issuingMiningRounds),
							cantonproto.Field("validatorRights", &v2.Value{
								Sum: &v2.Value_GenMap{GenMap: &v2.GenMap{Entries: []*v2.GenMap_Entry{}}},
							}),
							cantonproto.Field("featuredAppRight", &v2.Value{
								Sum: &v2.Value_Optional{Optional: &v2.Optional{}},
							}),
						)),
					)),
				),
			},
		},
	}

	return cmd, disclosedContracts, nil
}

func buildTokenStandardTransferCommand(
	args xcbuilder.TransferArgs,
	transferPackageID string,
	factoryContractID string,
	choiceContextData map[string]any,
	senderHoldings []*v2.ActiveContract,
	decimals int32,
	requestedAt time.Time,
	executeBefore time.Time,
) (*v2.Command, error) {
	contract, ok := args.GetContract()
	if !ok {
		return nil, fmt.Errorf("missing token contract")
	}
	instrumentAdmin, instrumentID, ok := parseCantonTokenContract(contract)
	if !ok {
		return nil, fmt.Errorf("invalid Canton token contract %q, expected <instrument-admin>#<instrument-id>", contract)
	}

	if factoryContractID == "" {
		return nil, fmt.Errorf("missing token transfer factory contract id for %s#%s", instrumentAdmin, instrumentID)
	}

	inputHoldingElements := make([]*v2.Value, 0, len(senderHoldings))
	for _, holding := range senderHoldings {
		created := holding.GetCreatedEvent()
		owner, holdingAdmin, holdingID, _, ok := extractTokenHoldingView(created)
		if !ok {
			continue
		}
		if owner != string(args.GetFrom()) || holdingAdmin != instrumentAdmin || holdingID != instrumentID {
			continue
		}
		inputHoldingElements = append(inputHoldingElements, cantonproto.ContractIDValue(created.GetContractId()))
	}
	if len(inputHoldingElements) == 0 {
		return nil, fmt.Errorf("no visible token holdings found for sender %s and %s#%s", args.GetFrom(), instrumentAdmin, instrumentID)
	}

	amountNumeric := transferAmountNumeric(args, decimals)
	choiceContextValue, err := tokenChoiceContextToValue(choiceContextData)
	if err != nil {
		return nil, fmt.Errorf("build token transfer choice context: %w", err)
	}

	return &v2.Command{
		Command: &v2.Command_Exercise{
			Exercise: &v2.ExerciseCommand{
				TemplateId: &v2.Identifier{
					PackageId:  transferPackageID,
					ModuleName: tokenTransferModule,
					EntityName: tokenTransferEntity,
				},
				ContractId: factoryContractID,
				Choice:     "TransferFactory_Transfer",
				ChoiceArgument: cantonproto.RecordValue(
					cantonproto.Field("expectedAdmin", cantonproto.PartyValue(instrumentAdmin)),
					cantonproto.Field("transfer", cantonproto.RecordValue(
						cantonproto.Field("sender", cantonproto.PartyValue(string(args.GetFrom()))),
						cantonproto.Field("receiver", cantonproto.PartyValue(string(args.GetTo()))),
						cantonproto.Field("amount", cantonproto.NumericValue(amountNumeric)),
						cantonproto.Field("instrumentId", cantonproto.RecordValue(
							cantonproto.Field("admin", cantonproto.PartyValue(instrumentAdmin)),
							cantonproto.Field("id", cantonproto.TextValue(instrumentID)),
						)),
						cantonproto.Field("requestedAt", &v2.Value{
							Sum: &v2.Value_Timestamp{Timestamp: requestedAt.UnixMicro()},
						}),
						cantonproto.Field("executeBefore", &v2.Value{
							Sum: &v2.Value_Timestamp{Timestamp: executeBefore.UnixMicro()},
						}),
						cantonproto.Field("inputHoldingCids", &v2.Value{
							Sum: &v2.Value_List{
								List: &v2.List{Elements: inputHoldingElements},
							},
						}),
						cantonproto.Field("meta", cantonproto.RecordValue(
							cantonproto.Field("values", &v2.Value{
								Sum: &v2.Value_TextMap{
									TextMap: &v2.TextMap{Entries: []*v2.TextMap_Entry{}},
								},
							}),
						)),
					)),
					cantonproto.Field("extraArgs", cantonproto.RecordValue(
						cantonproto.Field("context", choiceContextValue),
						cantonproto.Field("meta", cantonproto.RecordValue(
							cantonproto.Field("values", &v2.Value{
								Sum: &v2.Value_TextMap{
									TextMap: &v2.TextMap{Entries: []*v2.TextMap_Entry{}},
								},
							}),
						)),
					)),
				),
			},
		},
	}, nil
}

func buildTokenTransferInstructionAcceptCommand(
	transferPackageID string,
	contractID string,
	choiceContextData map[string]any,
) (*v2.Command, error) {
	choiceContextValue, err := tokenChoiceContextToValue(choiceContextData)
	if err != nil {
		return nil, fmt.Errorf("build token accept choice context: %w", err)
	}

	return &v2.Command{
		Command: &v2.Command_Exercise{
			Exercise: &v2.ExerciseCommand{
				TemplateId: &v2.Identifier{
					PackageId:  transferPackageID,
					ModuleName: "Splice.Api.Token.TransferInstructionV1",
					EntityName: "TransferInstruction",
				},
				ContractId: contractID,
				Choice:     "TransferInstruction_Accept",
				ChoiceArgument: cantonproto.RecordValue(
					cantonproto.Field("extraArgs", cantonproto.RecordValue(
						cantonproto.Field("context", choiceContextValue),
						cantonproto.Field("meta", cantonproto.RecordValue(
							cantonproto.Field("values", &v2.Value{
								Sum: &v2.Value_TextMap{
									TextMap: &v2.TextMap{Entries: []*v2.TextMap_Entry{}},
								},
							}),
						)),
					)),
				),
			},
		},
	}, nil
}

func buildAcceptedTransferOfferCompleteCommand(
	templateID *v2.Identifier,
	contractID string,
	senderPartyID string,
	amuletRulesID string,
	openMiningRoundID string,
	issuingMiningRoundID string,
	issuingMiningRoundNumber int64,
	transferInputs []*v2.Value,
) *v2.Command {
	issuingMiningRounds := &v2.Value{
		Sum: &v2.Value_GenMap{
			GenMap: &v2.GenMap{
				Entries: []*v2.GenMap_Entry{
					{
						Key: cantonproto.RecordValue(
							cantonproto.Field("number", &v2.Value{Sum: &v2.Value_Int64{Int64: issuingMiningRoundNumber}}),
						),
						Value: cantonproto.ContractIDValue(issuingMiningRoundID),
					},
				},
			},
		},
	}

	return &v2.Command{
		Command: &v2.Command_Exercise{
			Exercise: &v2.ExerciseCommand{
				TemplateId: templateID,
				ContractId: contractID,
				Choice:     "AcceptedTransferOffer_Complete",
				ChoiceArgument: cantonproto.RecordValue(
					cantonproto.Field("inputs", &v2.Value{
						Sum: &v2.Value_List{
							List: &v2.List{Elements: transferInputs},
						},
					}),
					cantonproto.Field("transferContext", cantonproto.RecordValue(
						cantonproto.Field("amuletRules", cantonproto.ContractIDValue(amuletRulesID)),
						cantonproto.Field("context", cantonproto.RecordValue(
							cantonproto.Field("openMiningRound", cantonproto.ContractIDValue(openMiningRoundID)),
							cantonproto.Field("issuingMiningRounds", issuingMiningRounds),
							cantonproto.Field("validatorRights", &v2.Value{
								Sum: &v2.Value_GenMap{
									GenMap: &v2.GenMap{Entries: []*v2.GenMap_Entry{}},
								},
							}),
							cantonproto.Field("featuredAppRight", &v2.Value{
								Sum: &v2.Value_Optional{Optional: &v2.Optional{}},
							}),
						)),
					)),
					cantonproto.Field("walletProvider", cantonproto.PartyValue(senderPartyID)),
				),
			},
		},
	}
}

func tokenDisclosedContractsToProto(disclosed []TokenRegistryDisclosedContract, packageMap map[string]string) ([]*v2.DisclosedContract, string, error) {
	contracts := make([]*v2.DisclosedContract, 0, len(disclosed))
	var synchronizerID string
	for _, item := range disclosed {
		templateID, err := templateRefToIdentifier(item.TemplateID, packageMap)
		if err != nil {
			return nil, "", err
		}
		blob, err := base64.StdEncoding.DecodeString(item.CreatedEventBlob)
		if err != nil {
			return nil, "", fmt.Errorf("decode disclosed contract blob for %s: %w", item.ContractID, err)
		}
		contracts = append(contracts, &v2.DisclosedContract{
			TemplateId:       templateID,
			ContractId:       item.ContractID,
			CreatedEventBlob: blob,
		})
		if synchronizerID == "" && item.SynchronizerID != "" {
			synchronizerID = item.SynchronizerID
		}
	}
	return contracts, synchronizerID, nil
}

func templateRefToIdentifier(ref string, packageMap map[string]string) (*v2.Identifier, error) {
	trimmed := strings.TrimPrefix(ref, "#")
	parts := strings.SplitN(trimmed, ":", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid template ref %q", ref)
	}
	packageID := parts[0]
	if strings.HasPrefix(ref, "#") {
		resolved, ok := packageMap[parts[0]]
		if !ok || resolved == "" {
			return nil, fmt.Errorf("no package id found for package name %q", parts[0])
		}
		packageID = resolved
	}
	return &v2.Identifier{
		PackageId:  packageID,
		ModuleName: parts[1],
		EntityName: parts[2],
	}, nil
}

func tokenChoiceContextToValue(contextData map[string]any) (*v2.Value, error) {
	valuesAny, ok := contextData["values"]
	if !ok || valuesAny == nil {
		return cantonproto.RecordValue(
			cantonproto.Field("values", &v2.Value{
				Sum: &v2.Value_TextMap{TextMap: &v2.TextMap{Entries: []*v2.TextMap_Entry{}}},
			}),
		), nil
	}

	valuesMap, ok := valuesAny.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("token choice context values must be an object, got %T", valuesAny)
	}

	entries := make([]*v2.TextMap_Entry, 0, len(valuesMap))
	for key, value := range valuesMap {
		converted, err := tokenAnyValueToProto(value)
		if err != nil {
			return nil, fmt.Errorf("convert token choice context value %q: %w", key, err)
		}
		entries = append(entries, &v2.TextMap_Entry{Key: key, Value: converted})
	}
	return cantonproto.RecordValue(
		cantonproto.Field("values", &v2.Value{
			Sum: &v2.Value_TextMap{TextMap: &v2.TextMap{Entries: entries}},
		}),
	), nil
}

func tokenAnyValueToProto(value any) (*v2.Value, error) {
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected AnyValue object, got %T", value)
	}
	tag, _ := obj["tag"].(string)
	switch tag {
	case "AV_Text":
		text, _ := obj["value"].(string)
		return &v2.Value{
			Sum: &v2.Value_Variant{Variant: &v2.Variant{
				Constructor: tag,
				Value:       cantonproto.TextValue(text),
			}},
		}, nil
	case "AV_Int":
		return &v2.Value{
			Sum: &v2.Value_Variant{Variant: &v2.Variant{
				Constructor: tag,
				Value:       &v2.Value{Sum: &v2.Value_Int64{Int64: tokenInt64Value(obj["value"])}},
			}},
		}, nil
	case "AV_Decimal":
		return &v2.Value{
			Sum: &v2.Value_Variant{Variant: &v2.Variant{
				Constructor: tag,
				Value:       cantonproto.NumericValue(tokenStringValue(obj["value"])),
			}},
		}, nil
	case "AV_Bool":
		boolValue, _ := obj["value"].(bool)
		return &v2.Value{
			Sum: &v2.Value_Variant{Variant: &v2.Variant{
				Constructor: tag,
				Value:       &v2.Value{Sum: &v2.Value_Bool{Bool: boolValue}},
			}},
		}, nil
	case "AV_Time":
		timestamp, err := time.Parse(time.RFC3339Nano, tokenStringValue(obj["value"]))
		if err != nil {
			return nil, fmt.Errorf("parse AV_Time: %w", err)
		}
		return &v2.Value{
			Sum: &v2.Value_Variant{Variant: &v2.Variant{
				Constructor: tag,
				Value:       &v2.Value{Sum: &v2.Value_Timestamp{Timestamp: timestamp.UnixMicro()}},
			}},
		}, nil
	case "AV_Party":
		return &v2.Value{
			Sum: &v2.Value_Variant{Variant: &v2.Variant{
				Constructor: tag,
				Value:       cantonproto.PartyValue(tokenStringValue(obj["value"])),
			}},
		}, nil
	case "AV_ContractId":
		return &v2.Value{
			Sum: &v2.Value_Variant{Variant: &v2.Variant{
				Constructor: tag,
				Value:       cantonproto.ContractIDValue(tokenStringValue(obj["value"])),
			}},
		}, nil
	case "AV_List":
		rawList, ok := obj["value"].([]any)
		if !ok {
			return nil, fmt.Errorf("AV_List value must be an array, got %T", obj["value"])
		}
		elements := make([]*v2.Value, 0, len(rawList))
		for _, item := range rawList {
			converted, err := tokenAnyValueToProto(item)
			if err != nil {
				return nil, err
			}
			elements = append(elements, converted)
		}
		return &v2.Value{
			Sum: &v2.Value_Variant{Variant: &v2.Variant{
				Constructor: tag,
				Value:       &v2.Value{Sum: &v2.Value_List{List: &v2.List{Elements: elements}}},
			}},
		}, nil
	case "AV_Map":
		rawMap, ok := obj["value"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("AV_Map value must be an object, got %T", obj["value"])
		}
		entries := make([]*v2.TextMap_Entry, 0, len(rawMap))
		for key, item := range rawMap {
			converted, err := tokenAnyValueToProto(item)
			if err != nil {
				return nil, err
			}
			entries = append(entries, &v2.TextMap_Entry{Key: key, Value: converted})
		}
		return &v2.Value{
			Sum: &v2.Value_Variant{Variant: &v2.Variant{
				Constructor: tag,
				Value:       &v2.Value{Sum: &v2.Value_TextMap{TextMap: &v2.TextMap{Entries: entries}}},
			}},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported AnyValue tag %q", tag)
	}
}

func tokenInt64Value(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		n, _ := typed.Int64()
		return n
	default:
		n, _ := strconv.ParseInt(tokenStringValue(value), 10, 64)
		return n
	}
}

func tokenStringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", value)
	}
}
