package client

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	xcbuilder "github.com/cordialsys/crosschain/builder"
	cantonproto "github.com/cordialsys/crosschain/chain/canton/types"
	v2 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2"
)

func transferAmountNumeric(args xcbuilder.TransferArgs, decimals int32) string {
	amount := args.GetAmount()
	return amount.ToHuman(decimals).String()
}

func buildTransferOfferCreateCommand(args xcbuilder.TransferArgs, amuletRules AmuletRules, commandID string, decimals int32) *v2.Command {
	amountNumeric := transferAmountNumeric(args, decimals)
	packageID := amuletRules.GetSpliceId()
	return &v2.Command{
		Command: &v2.Command_Create{
			Create: &v2.CreateCommand{
				TemplateId: &v2.Identifier{
					PackageId:  packageID,
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
