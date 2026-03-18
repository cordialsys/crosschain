package client

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
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
	v2 "github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2"
	"github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/admin"
	"github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
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

// SubmitTx accepts either a serialized Canton transfer submission or a
// serialized Canton create-account step and dispatches it accordingly.
func (client *Client) SubmitTx(ctx context.Context, submitReq xctypes.SubmitTxReq) error {
	if len(submitReq.TxData) == 0 {
		return fmt.Errorf("empty transaction data")
	}

	if createAccountInput, err := tx_input.ParseCreateAccountInput(submitReq.TxData); err == nil {
		return client.CreateAccount(ctx, createAccountInput)
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

// TxFromInput builds a Tx from a TxInput and the transfer args, validating the hash and contents.
func TxFromInput(args xcbuilder.TransferArgs, input *tx_input.TxInput, decimals int32) (*cantontx.Tx, error) {
	return cantontx.NewTx(input, args, decimals)
}

var _ xclient.AccountClient = &Client{}

// FetchCreateAccountInput fetches all on-chain data required to register a Canton external party
// and advances all registration steps that do not require an explicit external
// signature. If another signed step is needed, it returns the payload for that
// step; otherwise it returns nil to signal that registration is complete.
func (client *Client) FetchCreateAccountInput(ctx context.Context, args *xclient.CreateAccountArgs) (xclient.CreateAccountInput, error) {
	publicKeyBytes := args.GetPublicKey()
	partyID := string(args.GetAddress())

	exists, err := client.ledgerClient.ExternalPartyExists(ctx, partyID)
	if err != nil {
		return nil, fmt.Errorf("failed to check external party registration: %w", err)
	}
	if !exists {
		authCtx := client.ledgerClient.authCtx(ctx)
		partyHint := hex.EncodeToString(publicKeyBytes)
		signingPubKey := &v2.SigningPublicKey{
			Format:  v2.CryptoKeyFormat_CRYPTO_KEY_FORMAT_RAW,
			KeyData: publicKeyBytes,
			KeySpec: v2.SigningKeySpec_SIGNING_KEY_SPEC_EC_CURVE25519,
		}

		topologyResp, err := client.ledgerClient.adminClient.GenerateExternalPartyTopology(authCtx, &admin.GenerateExternalPartyTopologyRequest{
			Synchronizer: TestnetSynchronizerID,
			PartyHint:    partyHint,
			PublicKey:    signingPubKey,
		})
		if err != nil {
			return nil, fmt.Errorf("GenerateExternalPartyTopology failed: %w", err)
		}

		txns := make([][]byte, 0, len(topologyResp.GetTopologyTransactions()))
		for _, txBytes := range topologyResp.GetTopologyTransactions() {
			txns = append(txns, txBytes)
		}

		input := &tx_input.CreateAccountInput{
			Stage:                tx_input.CreateAccountStageAllocate,
			Description:          "Sign signature_request.payload, append the raw signature hex to create_account_input, then submit the combined hex with `xc submit --chain canton <combined_hex>`.",
			PartyID:              partyID,
			PublicKeyFingerprint: topologyResp.GetPublicKeyFingerprint(),
			TopologyMultiHash:    topologyResp.GetMultiHash(),
			TopologyTransactions: txns,
		}

		if err := input.VerifySignaturePayloads(); err != nil {
			return nil, fmt.Errorf("hash verification failed after fetch: %w", err)
		}
		return input, nil
	}

	if err := client.ledgerClient.CreateUser(ctx, partyID); err != nil {
		return nil, fmt.Errorf("CreateUser failed: %w", err)
	}
	if err := client.ledgerClient.CreateExternalPartySetupProposal(ctx, partyID); err != nil {
		return nil, fmt.Errorf("CreateExternalPartySetupProposal failed: %w", err)
	}

	ledgerEnd, err := client.ledgerClient.GetLedgerEnd(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledger end: %w", err)
	}
	contracts, err := client.ledgerClient.GetActiveContracts(ctx, partyID, ledgerEnd, true)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active contracts: %w", err)
	}
	if client.ledgerClient.HasTransferPreapprovalContract(ctx, contracts) {
		return nil, nil
	}

	for _, contract := range contracts {
		event := contract.GetCreatedEvent()
		if event == nil {
			continue
		}
		tid := event.GetTemplateId()
		if tid == nil || tid.GetEntityName() != "ExternalPartySetupProposal" {
			continue
		}

		cmd := &v2.Command{
			Command: &v2.Command_Exercise{
				Exercise: &v2.ExerciseCommand{
					TemplateId:     tid,
					ContractId:     event.GetContractId(),
					Choice:         "ExternalPartySetupProposal_Accept",
					ChoiceArgument: &v2.Value{Sum: &v2.Value_Record{Record: &v2.Record{}}},
				},
			},
		}
		commandID := newRegisterCommandId()
		prepareResp, err := client.ledgerClient.PrepareSubmissionRequest(ctx, cmd, commandID, partyID)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare ExternalPartySetupProposal_Accept: %w", err)
		}
		preparedTxBz, err := proto.Marshal(prepareResp.GetPreparedTransaction())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal setup proposal prepared transaction: %w", err)
		}

		input := &tx_input.CreateAccountInput{
			Stage:                            tx_input.CreateAccountStageAccept,
			Description:                      "Sign signature_request.payload, append the raw signature hex to create_account_input, then submit the combined hex with `xc submit --chain canton <combined_hex>`.",
			PartyID:                          partyID,
			SetupProposalPreparedTransaction: preparedTxBz,
			SetupProposalHash:                prepareResp.GetPreparedTransactionHash(),
			SetupProposalHashing:             prepareResp.GetHashingSchemeVersion(),
			SetupProposalSubmissionID:        newRegisterCommandId(),
		}
		if err := input.VerifySignaturePayloads(); err != nil {
			return nil, fmt.Errorf("hash verification failed after fetch: %w", err)
		}
		return input, nil
	}

	return nil, nil
}

// CreateAccount submits the signed Canton account-registration step described by
// the serialized CreateAccountInput.
func (client *Client) CreateAccount(ctx context.Context, createInput xclient.CreateAccountInput) error {
	cantonInput, ok := createInput.(*tx_input.CreateAccountInput)
	if !ok {
		return fmt.Errorf("invalid CreateAccountInput type for Canton")
	}
	if len(cantonInput.Signature) == 0 {
		return fmt.Errorf("CreateAccountInput has not been signed; call SetSignatures first")
	}
	if err := cantonInput.VerifySignaturePayloads(); err != nil {
		return fmt.Errorf("invalid CreateAccountInput: %w", err)
	}

	authCtx := client.ledgerClient.authCtx(ctx)

	switch cantonInput.Stage {
	case tx_input.CreateAccountStageAllocate:
		txns := make([]*admin.AllocateExternalPartyRequest_SignedTransaction, 0, len(cantonInput.TopologyTransactions))
		for _, txBytes := range cantonInput.TopologyTransactions {
			txns = append(txns, &admin.AllocateExternalPartyRequest_SignedTransaction{Transaction: txBytes})
		}
		sig := &v2.Signature{
			Format:               v2.SignatureFormat_SIGNATURE_FORMAT_RAW,
			Signature:            cantonInput.Signature,
			SignedBy:             cantonInput.PublicKeyFingerprint,
			SigningAlgorithmSpec: v2.SigningAlgorithmSpec_SIGNING_ALGORITHM_SPEC_ED25519,
		}
		_, err := client.ledgerClient.adminClient.AllocateExternalParty(authCtx, &admin.AllocateExternalPartyRequest{
			Synchronizer:           TestnetSynchronizerID,
			OnboardingTransactions: txns,
			MultiHashSignatures:    []*v2.Signature{sig},
		})
		if err != nil && !isAlreadyExists(err) {
			return fmt.Errorf("AllocateExternalParty failed: %w", err)
		}
		return nil
	case tx_input.CreateAccountStageAccept:
		var preparedTx interactive.PreparedTransaction
		if err := proto.Unmarshal(cantonInput.SetupProposalPreparedTransaction, &preparedTx); err != nil {
			return fmt.Errorf("failed to unmarshal setup proposal prepared transaction: %w", err)
		}
		_, keyFingerprint, err := cantonaddress.ParsePartyID(xc.Address(cantonInput.PartyID))
		if err != nil {
			return fmt.Errorf("failed to parse party ID for setup proposal accept: %w", err)
		}
		executeReq := &interactive.ExecuteSubmissionAndWaitRequest{
			PreparedTransaction: &preparedTx,
			PartySignatures: &interactive.PartySignatures{
				Signatures: []*interactive.SinglePartySignatures{
					{
						Party: cantonInput.PartyID,
						Signatures: []*v2.Signature{
							{
								Format:               v2.SignatureFormat_SIGNATURE_FORMAT_RAW,
								Signature:            cantonInput.Signature,
								SignedBy:             keyFingerprint,
								SigningAlgorithmSpec: v2.SigningAlgorithmSpec_SIGNING_ALGORITHM_SPEC_ED25519,
							},
						},
					},
				},
			},
			DeduplicationPeriod: &interactive.ExecuteSubmissionAndWaitRequest_DeduplicationDuration{
				DeduplicationDuration: durationpb.New(300 * time.Second),
			},
			SubmissionId:         cantonInput.SetupProposalSubmissionID,
			HashingSchemeVersion: cantonInput.SetupProposalHashing,
		}
		_, err = client.ledgerClient.interactiveSubmissionClient.ExecuteSubmissionAndWait(authCtx, executeReq)
		if err != nil && !isAlreadyExists(err) {
			return fmt.Errorf("ExternalPartySetupProposal_Accept failed: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported create-account stage %q", cantonInput.Stage)
	}
}
