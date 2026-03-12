package client

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	cantonaddress "github.com/cordialsys/crosschain/chain/canton/address"
	cantonkc "github.com/cordialsys/crosschain/chain/canton/keycloak"
	cantontx "github.com/cordialsys/crosschain/chain/canton/tx"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/cosmos/gogoproto/proto"
	v2 "github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2"
	"github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive"
	"github.com/sirupsen/logrus"
)


// Client for Canton using the gRPC Ledger API
type Client struct {
	Asset *xc.ChainConfig

	ledgerClient *GrpcLedgerClient

	// adminKC fetches operator-level tokens (client_credentials grant).
	adminKC *cantonkc.Client
	// walletKC acquires canton-ui tokens for scan proxy HTTP calls.
	walletKC *cantonkc.Client

	cantonUiUsername string
	cantonUiPassword string
}

var _ xclient.Client = &Client{}

// NewClient returns a new Canton gRPC Client
func NewClient(cfgI *xc.ChainConfig) (*Client, error) {
	cfg := cfgI.GetChain()

	if cfg.URL == "" {
		return nil, fmt.Errorf("no URL configured for Canton client")
	}

	keycloakURL, err := cantonEnv("CANTON_KEYCLOAK_URL")
	if err != nil {
		return nil, err
	}
	keycloakRealm, err := cantonEnv("CANTON_KEYCLOAK_REALM")
	if err != nil {
		return nil, err
	}
	adminClientID, err := cantonEnv("CANTON_VALIDATOR_ID")
	if err != nil {
		return nil, err
	}
	adminClientSecret, err := cantonEnv("CANTON_VALIDATOR_SECRET")
	if err != nil {
		return nil, err
	}
	cantonUiUsername, err := cantonEnv("CANTON_UI_ID")
	if err != nil {
		return nil, err
	}
	cantonUiPassword, err := cantonEnv("CANTON_UI_PASSWORD")
	if err != nil {
		return nil, err
	}

	client := &Client{
		Asset:            cfgI,
		adminKC:          cantonkc.NewClient(keycloakURL, keycloakRealm, adminClientID, adminClientSecret),
		walletKC:         cantonkc.NewClient(keycloakURL, keycloakRealm, adminClientID, adminClientSecret),
		cantonUiUsername: cantonUiUsername,
		cantonUiPassword: cantonUiPassword,
	}

	authToken, err := client.adminKC.AdminToken(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch auth token: %w", err)
	}
	if authToken == "" {
		return nil, errors.New("invalid authToken")
	}

	grpcClient, err := NewGrpcLedgerClient(cfg.URL, authToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create GrpcLedgerClient: %w", err)
	}
	client.ledgerClient = grpcClient

	return client, nil
}

// cantonUIToken acquires a canton-ui Keycloak token used for scan proxy HTTP calls.
func (client *Client) cantonUIToken(ctx context.Context) (string, error) {
	resp, err := client.walletKC.AcquireCantonUiToken(ctx, client.cantonUiUsername, client.cantonUiPassword)
	if err != nil {
		return "", fmt.Errorf("failed to acquire canton-ui token: %w", err)
	}
	return resp.AccessToken, nil
}

func (client *Client) PrepareTransferOfferCommand(ctx context.Context, args xcbuilder.TransferArgs, amuletRules AmuletRules) (*interactive.PrepareSubmissionResponse, error) {
	// amount := args.GetAmount()
	// amountStr := amount.ToHuman(10)
	commandID := newRegisterCommandId()
	cmd := &v2.Command{
		Command: &v2.Command_Create{
			Create: &v2.CreateCommand{
				TemplateId: &v2.Identifier{
					// TODO: Fetch via KnownPackages and match on splice-wallet version from amulet rules
					PackageId:  "fd57252dda29e3ce90028114c91b521cb661df5a9d6e87c41a9e91518215fa5b",
					ModuleName: "Splice.Wallet.TransferOffer",
					EntityName: "TransferOffer",
				},
				CreateArguments: &v2.Record{
					Fields: []*v2.RecordField{
						{
							Label: "sender",
							Value: &v2.Value{Sum: &v2.Value_Party{Party: string(args.GetFrom())}},
						},
						{
							Label: "receiver",
							Value: &v2.Value{Sum: &v2.Value_Party{Party: string(args.GetTo())}},
						},
						{
							Label: "dso",
							Value: &v2.Value{Sum: &v2.Value_Party{Party: amuletRules.AmuletRulesUpdate.Contract.Payload.DSO}},
						},
						{
							Label: "amount",
							Value: &v2.Value{
								Sum: &v2.Value_Record{
									Record: &v2.Record{
										Fields: []*v2.RecordField{
											{
												Label: "amount",
												Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: "10.0"}}, // hardcode for now
											},
											{
												Label: "unit",
												Value: &v2.Value{
													Sum: &v2.Value_Enum{
														Enum: &v2.Enum{
															Constructor: "AmuletUnit",
														},
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Label: "description",
							Value: &v2.Value{Sum: &v2.Value_Text{Text: ""}},
						},
						{
							Label: "expiresAt",
							Value: &v2.Value{Sum: &v2.Value_Timestamp{Timestamp: time.Now().UTC().Add(24 * time.Hour).UnixMicro()}},
						},
						{
							Label: "trackingId",
							Value: &v2.Value{Sum: &v2.Value_Text{Text: commandID}},
						},
					},
				},
			},
		},
	}

	prepareResp, err := client.ledgerClient.PrepareSubmissionRequest(ctx, cmd, commandID, string(args.GetFrom()))
	if err != nil {
		return nil, fmt.Errorf("failed to prepare submission for party setup proposal accept: %w", err)
	}

	return prepareResp, nil
}

// PrepareTransferPreapprovalCommand prepares an exercise of TransferPreapproval_Send on the
// recipient's TransferPreapproval contract. This is the flow used when the recipient is an
// external party that has completed setup (i.e. has a TransferPreapproval contract on the ledger).
//
// The sender exercises the choice directly, providing their amulet inputs and the transfer context.
func (client *Client) PrepareTransferPreapprovalCommand(
	ctx context.Context,
	args xcbuilder.TransferArgs,
	amuletRules AmuletRules,
	openMiningRound *RoundEntry,
	issuingMiningRound *RoundEntry,
	senderContracts []*v2.ActiveContract,
	recipientContracts []*v2.ActiveContract,
) (*interactive.PrepareSubmissionResponse, error) {
	senderPartyID := string(args.GetFrom())

	// Find the recipient's TransferPreapproval contract.
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
		return nil, fmt.Errorf("no TransferPreapproval contract found for recipient %s", args.GetTo())
	}

	// Build sender's amulet inputs.
	transferInputs := make([]*v2.Value, 0)
	for _, ac := range senderContracts {
		event := ac.GetCreatedEvent()
		if event == nil {
			continue
		}
		if event.GetTemplateId().GetEntityName() != "Amulet" {
			continue
		}
		transferInputs = append(transferInputs, &v2.Value{
			Sum: &v2.Value_Variant{
				Variant: &v2.Variant{
					Constructor: "InputAmulet",
					Value: &v2.Value{
						Sum: &v2.Value_ContractId{
							ContractId: event.GetContractId(),
						},
					},
				},
			},
		})
	}

	amuletRulesID := amuletRules.AmuletRulesUpdate.Contract.ContractID
	openMiningRoundID := openMiningRound.Contract.ContractID

	rn, err := strconv.ParseInt(issuingMiningRound.Contract.Payload.Round.Number, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issuing mining round number: %w", err)
	}
	issuingMiningRounds := &v2.Value{
		Sum: &v2.Value_GenMap{
			GenMap: &v2.GenMap{
				Entries: []*v2.GenMap_Entry{
					{
						Key: &v2.Value{
							Sum: &v2.Value_Record{
								Record: &v2.Record{
									Fields: []*v2.RecordField{
										{
											Label: "number",
											Value: &v2.Value{Sum: &v2.Value_Int64{Int64: rn}},
										},
									},
								},
							},
						},
						Value: &v2.Value{
							Sum: &v2.Value_ContractId{
								ContractId: issuingMiningRound.Contract.ContractID,
							},
						},
					},
				},
			},
		},
	}

	// Build disclosed contracts: TransferPreapproval + sender amulets + AmuletRules + OpenMiningRound + IssuingMiningRound.
	disclosedContracts := make([]*v2.DisclosedContract, 0)

	// Disclose the recipient's TransferPreapproval contract (the one being exercised).
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
		disclosedContracts = append(disclosedContracts, &v2.DisclosedContract{
			TemplateId:       event.GetTemplateId(),
			ContractId:       event.GetContractId(),
			CreatedEventBlob: event.GetCreatedEventBlob(),
		})
	}

	// Disclose AmuletRules.
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

	cmd := &v2.Command{
		Command: &v2.Command_Exercise{
			Exercise: &v2.ExerciseCommand{
				TemplateId: preapprovalTemplateID,
				ContractId: preapprovalContractID,
				Choice:     "TransferPreapproval_Send",
				ChoiceArgument: &v2.Value{
					Sum: &v2.Value_Record{
						Record: &v2.Record{
							Fields: []*v2.RecordField{
								{
									Label: "sender",
									Value: &v2.Value{Sum: &v2.Value_Party{Party: senderPartyID}},
								},
								{
									Label: "inputs",
									Value: &v2.Value{
										Sum: &v2.Value_List{
											List: &v2.List{Elements: transferInputs},
										},
									},
								},
								{
									Label: "amount",
									Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: "10.0"}},
								},
								{
									Label: "context",
									Value: &v2.Value{
										Sum: &v2.Value_Record{
											Record: &v2.Record{
												Fields: []*v2.RecordField{
													{
														Label: "amuletRules",
														Value: &v2.Value{Sum: &v2.Value_ContractId{ContractId: amuletRulesID}},
													},
													{
														Label: "context",
														Value: &v2.Value{
															Sum: &v2.Value_Record{
																Record: &v2.Record{
																	Fields: []*v2.RecordField{
																		{
																			Label: "openMiningRound",
																			Value: &v2.Value{Sum: &v2.Value_ContractId{ContractId: openMiningRoundID}},
																		},
																		{
																			Label: "issuingMiningRounds",
																			Value: issuingMiningRounds,
																		},
																		{
																			Label: "validatorRights",
																			Value: &v2.Value{
																				Sum: &v2.Value_GenMap{
																					GenMap: &v2.GenMap{Entries: []*v2.GenMap_Entry{}},
																				},
																			},
																		},
																		{
																			Label: "featuredAppRight",
																			Value: &v2.Value{
																				Sum: &v2.Value_Optional{Optional: &v2.Optional{}},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	commandID := newRegisterCommandId()
	prepareReq := &interactive.PrepareSubmissionRequest{
		CommandId:          commandID,
		Commands:           []*v2.Command{cmd},
		ActAs:              []string{senderPartyID},
		ReadAs:             []string{senderPartyID, ValidatorPartyId},
		SynchronizerId:     TestnetSynchronizerID,
		DisclosedContracts: disclosedContracts,
		VerboseHashing:     false,
	}

	authCtx := client.ledgerClient.authCtx(ctx)
	prepareResp, err := client.ledgerClient.interactiveSubmissionClient.PrepareSubmission(authCtx, prepareReq)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare TransferPreapproval_Send: %w", err)
	}

	return prepareResp, nil
}

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	input := tx_input.NewTxInput()
	from := args.GetFrom()
	to := args.GetTo()

	ledgerEnd, err := client.ledgerClient.GetLedgerEnd(ctx)
	if err != nil {
		return input, fmt.Errorf("failed to get ledger end: %w", err)
	}

	senderContracts, err := client.ledgerClient.GetActiveContracts(ctx, string(from), ledgerEnd, true)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active contracts: %w", err)
	}

	// Check if the recipient has a TransferPreapproval contract.
	// includeBlobs: true so the CreatedEventBlob is available for disclosure.
	recipientContracts, err := client.ledgerClient.GetActiveContracts(ctx, string(to), ledgerEnd, true)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recipient active contracts: %w", err)
	}
	isExternal := client.ledgerClient.HasTransferPreapprovalContract(ctx, recipientContracts)
	input.IsExternalTransfer = isExternal

	uiToken, err := client.cantonUIToken(ctx)
	if err != nil {
		return nil, err
	}

	amuletRules, err := client.ledgerClient.GetAmuletRules(ctx, uiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch amulet rules: %w", err)
	}

	var resp *interactive.PrepareSubmissionResponse
	if isExternal {
		openMiningRound, issuingMiningRound, err := client.ledgerClient.GetOpenAndIssuingMiningRound(ctx, uiToken)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch mining rounds: %w", err)
		}
		resp, err = client.PrepareTransferPreapprovalCommand(ctx, args, *amuletRules, openMiningRound, issuingMiningRound, senderContracts, recipientContracts)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare transfer preapproval command: %w", err)
		}
	} else {
		resp, err = client.PrepareTransferOfferCommand(ctx, args, *amuletRules)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare offer command: %w", err)
		}
	}

	input.PreparedTransaction = *resp.GetPreparedTransaction()
	input.Sighash = resp.GetPreparedTransactionHash()
	input.SubmissionId = NewCommandId()
	input.HashingSchemeVersion = resp.GetHashingSchemeVersion()

	return input, nil
}

// FetchLegacyTxInput - Deprecated, use FetchTransferInput
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	chainCfg := client.Asset.GetChain().Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx unmarshals the serialized ExecuteSubmissionRequest proto and calls
// InteractiveSubmissionService.ExecuteSubmissionAndWait
func (client *Client) SubmitTx(ctx context.Context, submitReq xctypes.SubmitTxReq) error {
	if len(submitReq.TxData) == 0 {
		return fmt.Errorf("empty transaction data")
	}

	var req interactive.ExecuteSubmissionRequest
	if err := proto.Unmarshal(submitReq.TxData, &req); err != nil {
		return fmt.Errorf("failed to unmarshal Canton execute request: %w", err)
	}

	andWaitReq := &interactive.ExecuteSubmissionAndWaitRequest{
		PreparedTransaction:  req.PreparedTransaction,
		PartySignatures:      req.PartySignatures,
		SubmissionId:         req.SubmissionId,
		UserId:               req.UserId,
		HashingSchemeVersion: req.HashingSchemeVersion,
	}
	// Convert deduplication period (unexported oneof interface - handle each concrete type)
	switch v := req.DeduplicationPeriod.(type) {
	case *interactive.ExecuteSubmissionRequest_DeduplicationDuration:
		andWaitReq.DeduplicationPeriod = &interactive.ExecuteSubmissionAndWaitRequest_DeduplicationDuration{
			DeduplicationDuration: v.DeduplicationDuration,
		}
	case *interactive.ExecuteSubmissionRequest_DeduplicationOffset:
		andWaitReq.DeduplicationPeriod = &interactive.ExecuteSubmissionAndWaitRequest_DeduplicationOffset{
			DeduplicationOffset: v.DeduplicationOffset,
		}
	}

	parties := []string{}
	if req.PartySignatures != nil {
		for _, ps := range req.PartySignatures.GetSignatures() {
			parties = append(parties, ps.GetParty())
		}
	}
	logrus.WithFields(logrus.Fields{
		"rpc":           "ExecuteSubmissionAndWait",
		"submission_id": req.SubmissionId,
		"parties":       parties,
	}).Trace("canton request")

	actx := client.ledgerClient.authCtx(ctx)
	_, err := client.ledgerClient.interactiveSubmissionClient.ExecuteSubmissionAndWait(actx, andWaitReq)
	if err != nil {
		return fmt.Errorf("failed to submit Canton transaction: %w", err)
	}
	logrus.WithField("submission_id", req.SubmissionId).Trace("canton response: ExecuteSubmissionAndWait accepted")
	return err
}

// FetchLegacyTxInfo - not implemented for Canton
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	return txinfo.LegacyTxInfo{}, errors.New("not implemented")
}

// FetchTxInfo fetches and normalizes transaction info for a Canton update by its updateId.
// The txHash must be the Canton updateId returned after submission.
func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	updateId := string(args.TxHash())
	chainCfg := client.Asset.GetChain()
	decimals := chainCfg.Decimals

	// Use admin token for lookup — the updateId is ledger-global
	resp, err := client.ledgerClient.GetUpdateById(ctx, updateId)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to fetch update: %w", err)
	}

	tx := resp.GetTransaction()
	if tx == nil {
		return txinfo.TxInfo{}, fmt.Errorf("update %s is not a transaction", updateId)
	}

	ledgerEnd, err := client.ledgerClient.GetLedgerEnd(ctx)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to get ledger end: %w", err)
	}

	txOffset := tx.GetOffset()
	var confirmations uint64
	if ledgerEnd > txOffset {
		confirmations = uint64(ledgerEnd - txOffset)
	}

	var blockTime time.Time
	if ts := tx.GetEffectiveAt(); ts != nil {
		blockTime = ts.AsTime()
	}
	block := txinfo.NewBlock(chainCfg.Chain, uint64(txOffset), tx.GetSynchronizerId(), blockTime)
	txInfo := txinfo.NewTxInfo(block, client.Asset, updateId, confirmations, nil)

	// Scan events: find the acting party (sender) from exercised events and
	// new Amulet owner (receiver) + amount from created Amulet events.
	var senderParty string
	type amuletCreation struct {
		owner  string
		amount xc.AmountBlockchain
	}
	var amuletCreations []amuletCreation

	for _, event := range tx.GetEvents() {
		if ex := event.GetExercised(); ex != nil {
			// The acting party on transfer-related choices is the sender
			if len(ex.GetActingParties()) > 0 && senderParty == "" {
				senderParty = ex.GetActingParties()[0]
			}
		}
		if cr := event.GetCreated(); cr != nil {
			tid := cr.GetTemplateId()
			if tid == nil || !isAmuletTemplate(tid) {
				continue
			}
			createArgs := cr.GetCreateArguments()
			if createArgs == nil {
				continue
			}
			// Extract owner and initialAmount from Amulet contract
			var owner string
			for _, f := range createArgs.GetFields() {
				if f.GetLabel() == "owner" {
					owner = f.GetValue().GetParty()
				}
			}
			if owner == "" {
				continue
			}
			bal, ok := ExtractAmuletBalance(createArgs, decimals)
			if !ok {
				continue
			}
			amuletCreations = append(amuletCreations, amuletCreation{owner: owner, amount: bal})
		}
	}

	// Build movements: one per new Amulet contract created for a non-sender owner
	for _, ac := range amuletCreations {
		if ac.owner == senderParty {
			// This is change going back to the sender, not the primary transfer destination
			continue
		}
		from := xc.Address(senderParty)
		to := xc.Address(ac.owner)
		txInfo.AddSimpleTransfer(from, to, "", ac.amount, nil, "")
	}

	// If we couldn't distinguish sender from receiver (e.g. self-transfer or change only),
	// fall back to reporting all creations
	if len(txInfo.Movements) == 0 && len(amuletCreations) > 0 {
		for _, ac := range amuletCreations {
			from := xc.Address(senderParty)
			to := xc.Address(ac.owner)
			txInfo.AddSimpleTransfer(from, to, "", ac.amount, nil, "")
		}
	}

	txInfo.Fees = txInfo.CalculateFees()
	txInfo.SyncDeprecatedFields()
	return *txInfo, nil
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	if contract, ok := args.Contract(); ok {
		return zero, fmt.Errorf("token balance queries not yet supported for Canton, contract: %s", contract)
	}

	// Check if we want to onboard the client
	// Onboarding process:
	// 1. CreateExternalParty:
	//   - GenerateExternalPartyTopology
	//   - AllocateExternalParty
	// 2. Create keycloak user
	// 3. Set keycloak user attributes: `canton_party_id` and `canton_participant_aud`
	// 4. CRITICAL: Create ledger user, with userId equal to keycloak user id. This allows us to use
	//   keycloak tokens for ledger interactions
	// 5. CreateExternalPartySetupProposal allows external parties to receive funds. It's critical:
	//   - ExternalParties cannot accept transfer offers because they have very limited validator visibility
	//   - ExternalParties use TransferPreapproaval flow, which is different from vanilla offer/accept
	// 6. Accept CreateExternalPartySetupProposal
	//   - List user active contracts
	//   - Create Approval for the contract
	//   - Sign and submit
	// TODO: Refactor
	// All calls in this branch are properly recovering from "[User/Party/Contract]Exists" errors
	// It's really important that onboarding is resilient
	v := os.Getenv("CANTON_REGISTER_ON_BALANCE")
	if v == "true" {
		seed := os.Getenv("XC_PRIVATE_KEY")
		if len(seed) == 0 {
			return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("missing priv key for party registration")
		}
		seedBz, err := hex.DecodeString(seed)
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), errors.New("failed to read private key")
		}
		privKey := ed25519.NewKeyFromSeed(seedBz)
		publicKey := privKey.Public().(ed25519.PublicKey)
		err = client.ledgerClient.RegisterExternalParty(ctx, publicKey, privKey)
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("failed to register external party: %w", err)
		}

		address, err := cantonaddress.GetAddressFromPublicKey(publicKey)
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("failed to get address: %w", err)
		}

		if err := client.ledgerClient.CreateUser(ctx, string(address)); err != nil {
			return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("failed to create ledger user: %w", err)
		}

		err = client.ledgerClient.CreateExternalPartySetupProposal(ctx, string(address))
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("failed to create external party setup proposal: %w", err)
		}

		// ut, err := client.walletKC.AcquireUserToken(ctx, partyHint, userPassword)
		// if err != nil {
		// 	return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("failed to fetch user token for user %s: %w", partyHint, err)
		// }
		// client.ledgerClient.SetToken(ut.AccessToken)

		err = client.ledgerClient.AcceptExternalPartySetupProposal(ctx, string(address), privKey)
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("failed to accept external party setup proposal: %w", err)
		}

		uiToken, err := client.cantonUIToken(ctx)
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), err
		}
		amuletRules, err := client.ledgerClient.GetAmuletRules(ctx, uiToken)
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("failed to fetch amulet rules: %w", err)
		}
		openMiningRound, issuingMiningRound, err := client.ledgerClient.GetOpenAndIssuingMiningRound(ctx, uiToken)
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("failed to fetch open mining round: %w", err)
		}

		ledgerEnd, err := client.ledgerClient.GetLedgerEnd(ctx)
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("failed to get ledger end: %w", err)
		}
		contracts, err := client.ledgerClient.GetActiveContracts(ctx, string(address), ledgerEnd, true)
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("failed to fetch active contracts: %w", err)
		}
		err = client.ledgerClient.CompleteAcceptedTransferOffer(ctx, string(address), amuletRules, openMiningRound, issuingMiningRound, privKey, contracts)
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("failed to complete accepted transfer offer: %w", err)
		}
	}

	return client.FetchNativeBalance(ctx, args.Address())
}

func FilterToAmuletContracts(contracts []*v2.ActiveContract) []*v2.ActiveContract {
	amuletContracts := make([]*v2.ActiveContract, 0)
	for _, c := range contracts {
		created := c.GetCreatedEvent()
		if created == nil {
			continue
		}
		tid := created.GetTemplateId()
		if tid == nil || !isAmuletTemplate(tid) {
			continue
		}

		amuletContracts = append(amuletContracts, c)
	}
	return amuletContracts
}

// FetchNativeBalance fetches the native (Amulet/CC) balance for a Canton party
// by streaming all active contracts via gRPC StateService, then summing up
// contracts whose template belongs to Splice.Amulet.
func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	partyID := string(address)
	if partyID == "" {
		return zero, fmt.Errorf("empty address")
	}

	decimals := client.Asset.GetChain().Decimals
	ledgerEnd, err := client.ledgerClient.GetLedgerEnd(ctx)
	if err != nil {
		return zero, fmt.Errorf("failed to get ledger end: %w", err)
	}

	contracts, err := client.ledgerClient.GetActiveContracts(ctx, string(address), ledgerEnd, false)
	if err != nil {
		return zero, fmt.Errorf("failed to query active contracts for party %s: %w", partyID, err)
	}

	totalBalance := xc.NewAmountBlockchainFromUint64(0)
	for _, c := range contracts {
		created := c.GetCreatedEvent()
		if created == nil {
			continue
		}
		tid := created.GetTemplateId()
		if tid == nil || !isAmuletTemplate(tid) {
			continue
		}

		if bal, ok := ExtractAmuletBalance(created.GetCreateArguments(), decimals); ok {
			logrus.WithFields(logrus.Fields{
				KeyContractId:    created.GetContractId(),
				KeyInitialAmount: bal.String(),
				KeyRunningTotal:  totalBalance.String(),
			}).Trace("canton: Amulet contract balance")
			totalBalance = totalBalance.Add(&bal)
		}
	}

	return totalBalance, nil
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	return 0, errors.New("not implemented")
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	return &txinfo.BlockWithTransactions{}, errors.New("not implemented")
}

// KeyFingerprintFromAddress extracts the key fingerprint from a Canton party address
func KeyFingerprintFromAddress(addr xc.Address) (string, error) {
	_, fingerprint, err := cantonaddress.ParsePartyID(addr)
	if err != nil {
		return "", err
	}
	return fingerprint, nil
}

// TxFromInput builds a Tx from a TxInput and the transfer args
func TxFromInput(args xcbuilder.TransferArgs, input *tx_input.TxInput) (*cantontx.Tx, error) {
	fingerprint, err := KeyFingerprintFromAddress(args.GetFrom())
	if err != nil {
		return nil, fmt.Errorf("failed to parse sender party ID: %w", err)
	}
	return &cantontx.Tx{
		PreparedTransaction:     &input.PreparedTransaction,
		PreparedTransactionHash: input.Sighash,
		HashingSchemeVersion:    input.HashingSchemeVersion,
		Party:                   string(args.GetFrom()),
		KeyFingerprint:          fingerprint,
		SubmissionId:            input.SubmissionId,
	}, nil
}
