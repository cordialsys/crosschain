package client

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	cantonaddress "github.com/cordialsys/crosschain/chain/canton/address"
	cantonproto "github.com/cordialsys/crosschain/chain/canton/types"
	v2 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/admin"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	EntityAmulet       = "Amulet"
	EntityLockedAmulet = "LockedAmulet"
	KeyContractId      = "contract_id"
	KeyFilter          = "filter"
	KeyInitialAmount   = "initial_amount"
	KeyMethod          = "method"
	KeyOffset          = "offset"
	KeyParty           = "party"
	KeyRunningTotal    = "running_total"
	KeySynchronizerId  = "synchronizer_id"
	KeyTemplateId      = "template_id"
	KeyTxCount         = "tx_count"
	KeyUserId          = "user_id"
	LabelAmount        = "amount"
	LabelInitialAmount = "initialAmount"
	ModuleSpliceAmulet = "SpliceAmulet"
)

type runtimeIdentityConfig struct {
	validatorPartyID       string
	validatorServiceUserID string
	deduplicationWindow    time.Duration
	restAPIURL             string
	scanProxyURL           string
	scanAPIURL             string
}

type GrpcLedgerClient struct {
	// Bearer token injected into every gRPC call
	authToken                   string
	adminClient                 admin.PartyManagementServiceClient
	packageManagementClient     admin.PackageManagementServiceClient
	commandClient               v2.CommandServiceClient
	completionClient            v2.CommandCompletionServiceClient
	interactiveSubmissionClient interactive.InteractiveSubmissionServiceClient
	stateClient                 v2.StateServiceClient
	updateClient                v2.UpdateServiceClient
	userManagementClient        admin.UserManagementServiceClient
	validatorPartyID            string
	validatorServiceUserID      string
	deduplicationWindow         time.Duration
	restAPIURL                  string
	scanProxyURL                string
	scanAPIURL                  string
	httpClient                  *http.Client
	logger                      *logrus.Entry
}

type scanProxyRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body"`
}

type TokenChoiceContext struct {
	ChoiceContextData  map[string]any                   `json:"choiceContextData"`
	DisclosedContracts []TokenRegistryDisclosedContract `json:"disclosedContracts"`
}

type TokenRegistryDisclosedContract struct {
	TemplateID       string `json:"templateId"`
	ContractID       string `json:"contractId"`
	CreatedEventBlob string `json:"createdEventBlob"`
	SynchronizerID   string `json:"synchronizerId"`
}

type TokenTransferFactoryContext struct {
	FactoryID     string             `json:"factoryId"`
	TransferKind  string             `json:"transferKind"`
	ChoiceContext TokenChoiceContext `json:"choiceContext"`
}

type TokenInstrumentMetadata struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Symbol        string         `json:"symbol"`
	Decimals      int32          `json:"decimals"`
	SupportedAPIs map[string]any `json:"supportedApis"`
}

type TokenMetadataRegistryInfo struct {
	AdminID       string         `json:"adminId"`
	SupportedAPIs map[string]any `json:"supportedApis"`
}

func NewGrpcLedgerClient(target string, authToken string, cfg runtimeIdentityConfig) (*GrpcLedgerClient, error) {
	if authToken == "" {
		return nil, errors.New("GrpcLedgerClient requires a valid authToken")
	}

	// Determine TLS vs plain from URL scheme and derive the gRPC target
	// (gRPC targets don't include the scheme)
	var creds credentials.TransportCredentials
	if strings.HasPrefix(target, "https://") {
		target = strings.TrimPrefix(target, "https://")
		creds = credentials.NewTLS(&tls.Config{})
	} else if strings.HasPrefix(target, "http://") {
		target = strings.TrimPrefix(target, "http://")
		creds = insecure.NewCredentials()
	} else {
		// Assume TLS if no scheme
		creds = credentials.NewTLS(&tls.Config{})
	}
	target = strings.TrimRight(target, "/")
	if !strings.HasPrefix(target, "dns:///") {
		target = "dns:///" + target
	}

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to Canton: %w", err)
	}

	logger := logrus.NewEntry(logrus.New()).WithField("client", "GrpcLedgerClient")
	return &GrpcLedgerClient{
		authToken:                   authToken,
		adminClient:                 admin.NewPartyManagementServiceClient(conn),
		packageManagementClient:     admin.NewPackageManagementServiceClient(conn),
		stateClient:                 v2.NewStateServiceClient(conn),
		updateClient:                v2.NewUpdateServiceClient(conn),
		completionClient:            v2.NewCommandCompletionServiceClient(conn),
		interactiveSubmissionClient: interactive.NewInteractiveSubmissionServiceClient(conn),
		userManagementClient:        admin.NewUserManagementServiceClient(conn),
		commandClient:               v2.NewCommandServiceClient(conn),
		validatorPartyID:            cfg.validatorPartyID,
		validatorServiceUserID:      cfg.validatorServiceUserID,
		deduplicationWindow:         cantonproto.ResolveDeduplicationWindow(cfg.deduplicationWindow),
		restAPIURL:                  cfg.restAPIURL,
		scanProxyURL:                cfg.scanProxyURL,
		scanAPIURL:                  cfg.scanAPIURL,
		httpClient:                  http.DefaultClient,
		logger:                      logger,
	}, nil
}

func (c *GrpcLedgerClient) ResolvePackageIDByName(ctx context.Context, packageName string) (string, error) {
	if packageName == "" {
		return "", errors.New("empty required argument: packageName")
	}

	authCtx := c.authCtx(ctx)
	resp, err := c.packageManagementClient.ListKnownPackages(authCtx, &admin.ListKnownPackagesRequest{})
	if err != nil {
		return "", fmt.Errorf("list known packages: %w", err)
	}

	var latest *admin.PackageDetails
	for _, detail := range resp.GetPackageDetails() {
		if detail.GetName() != packageName {
			continue
		}
		if latest == nil || detail.GetKnownSince().AsTime().After(latest.GetKnownSince().AsTime()) {
			latest = detail
		}
	}
	if latest == nil {
		return "", fmt.Errorf("package not found: %s", packageName)
	}
	return latest.GetPackageId(), nil
}

func (c *GrpcLedgerClient) ListKnownPackageIDsByName(ctx context.Context) (map[string]string, error) {
	authCtx := c.authCtx(ctx)
	resp, err := c.packageManagementClient.ListKnownPackages(authCtx, &admin.ListKnownPackagesRequest{})
	if err != nil {
		return nil, fmt.Errorf("list known packages: %w", err)
	}

	type packageVersion struct {
		id         string
		knownSince time.Time
	}
	latest := make(map[string]packageVersion)
	for _, detail := range resp.GetPackageDetails() {
		if detail.GetName() == "" || detail.GetPackageId() == "" {
			continue
		}
		current, ok := latest[detail.GetName()]
		if !ok || detail.GetKnownSince().AsTime().After(current.knownSince) {
			latest[detail.GetName()] = packageVersion{
				id:         detail.GetPackageId(),
				knownSince: detail.GetKnownSince().AsTime(),
			}
		}
	}

	result := make(map[string]string, len(latest))
	for name, detail := range latest {
		result[name] = detail.id
	}
	return result, nil
}

// authCtx injects the Canton validator bearer token into the gRPC context.
func (c *GrpcLedgerClient) authCtx(ctx context.Context) context.Context {
	if c.authToken == "" {
		c.logger.Warn("empty authToken")
		return ctx
	}

	md := metadata.Pairs("authorization", "Bearer "+c.authToken)
	return metadata.NewOutgoingContext(ctx, md)
}

func (c *GrpcLedgerClient) GenerateExternalPartyTopology(ctx context.Context, req *admin.GenerateExternalPartyTopologyRequest) (*admin.GenerateExternalPartyTopologyResponse, error) {
	return c.adminClient.GenerateExternalPartyTopology(c.authCtx(ctx), req)
}

func (c *GrpcLedgerClient) AllocateExternalParty(ctx context.Context, req *admin.AllocateExternalPartyRequest) (*admin.AllocateExternalPartyResponse, error) {
	return c.adminClient.AllocateExternalParty(c.authCtx(ctx), req)
}

func (c *GrpcLedgerClient) PrepareSubmission(ctx context.Context, req *interactive.PrepareSubmissionRequest) (*interactive.PrepareSubmissionResponse, error) {
	return c.interactiveSubmissionClient.PrepareSubmission(c.authCtx(ctx), req)
}

func (c *GrpcLedgerClient) ExecuteSubmissionAndWait(ctx context.Context, req *interactive.ExecuteSubmissionAndWaitRequest) (*interactive.ExecuteSubmissionAndWaitResponse, error) {
	return c.interactiveSubmissionClient.ExecuteSubmissionAndWait(c.authCtx(ctx), req)
}

// getLedgerEnd fetches the current ledger end offset via gRPC StateService
func (c *GrpcLedgerClient) GetLedgerEnd(ctx context.Context) (int64, error) {
	authCtx := c.authCtx(ctx)
	logger := c.logger.WithField(KeyMethod, "GetLedgerEnd")
	logger.Trace("request")
	resp, err := c.stateClient.GetLedgerEnd(authCtx, &v2.GetLedgerEndRequest{})
	if err != nil {
		return 0, fmt.Errorf("failed to get ledger end: %w", err)
	}

	logger.WithField(KeyOffset, resp.GetOffset()).Trace("response")
	return resp.GetOffset(), nil
}

// getSynchronizerId fetches the synchronizer ID via GetConnectedSynchronizers
func (c *GrpcLedgerClient) GetSynchronizerId(ctx context.Context, party string) (string, error) {
	logger := c.logger.WithFields(logrus.Fields{
		KeyMethod: "GetConnectedSynchronizers",
		KeyParty:  party,
	})
	logger.Trace("request")

	authCtx := c.authCtx(ctx)
	resp, err := c.stateClient.GetConnectedSynchronizers(
		authCtx,
		&v2.GetConnectedSynchronizersRequest{Party: party},
	)
	if err != nil {
		return "", fmt.Errorf("failed to get connected synchronizers: %w", err)
	}

	syncs := resp.GetConnectedSynchronizers()
	if len(syncs) == 0 {
		return "", fmt.Errorf("no connected synchronizers found for party %s", party)
	}

	// Prefer one with SUBMISSION permission
	for _, s := range syncs {
		if s.GetPermission() == v2.ParticipantPermission_PARTICIPANT_PERMISSION_SUBMISSION {
			logrus.WithField(KeySynchronizerId, s.GetSynchronizerId()).Trace("selected synchronizer")
			return s.GetSynchronizerId(), nil
		}
	}

	logrus.WithField(KeySynchronizerId, syncs[0].GetSynchronizerId()).Warn("missing synchronizer with submission persmission, selecting fallback")
	return syncs[0].GetSynchronizerId(), nil
}

func (c *GrpcLedgerClient) ExternalPartyExists(ctx context.Context, partyID string) (bool, error) {
	if partyID == "" {
		return false, errors.New("empty required argument: partyID")
	}

	_, err := c.GetSynchronizerId(ctx, partyID)
	if err == nil {
		return true, nil
	}

	msg := err.Error()
	if strings.Contains(msg, "no connected synchronizers found") ||
		strings.Contains(msg, "PARTY_NOT_KNOWN") ||
		strings.Contains(msg, "party not known") ||
		strings.Contains(msg, "unknown party") ||
		strings.Contains(msg, "not found") {
		return false, nil
	}

	return false, err
}

func (c *GrpcLedgerClient) ResolveSynchronizerID(ctx context.Context, partyID string, fallback string) (string, error) {
	if partyID != "" {
		synchronizerID, err := c.GetSynchronizerId(ctx, partyID)
		if err == nil {
			return synchronizerID, nil
		}
		if fallback == "" {
			return "", err
		}
		c.logger.WithError(err).WithFields(logrus.Fields{
			KeyParty:          partyID,
			KeySynchronizerId: fallback,
		}).Warn("failed to resolve party synchronizer, using fallback")
		return fallback, nil
	}
	if fallback == "" {
		return "", errors.New("no synchronizer resolution inputs")
	}
	return fallback, nil
}

// Get active contracts for given party using StateServiceClient.GetActiveContracts
func (c *GrpcLedgerClient) GetActiveContracts(ctx context.Context, partyID string, ledgerEnd int64, includeBlobs bool) ([]*v2.ActiveContract, error) {
	if partyID == "" {
		return nil, errors.New("empty required argument: partyID")
	}
	logrus.WithFields(logrus.Fields{
		KeyMethod: "GetActiveContracts",
		KeyParty:  partyID,
		KeyOffset: ledgerEnd,
	}).Trace("canton request")

	req := &v2.GetActiveContractsRequest{
		ActiveAtOffset: ledgerEnd,
		EventFormat: &v2.EventFormat{
			// Verbose: true is required for createArguments to be populated on CreatedEvent
			Verbose: true,
			FiltersByParty: map[string]*v2.Filters{
				partyID: {
					Cumulative: []*v2.CumulativeFilter{{
						// Empty filter - we want to fetch all contracts
						IdentifierFilter: &v2.CumulativeFilter_WildcardFilter{
							WildcardFilter: &v2.WildcardFilter{
								IncludeCreatedEventBlob: includeBlobs,
							},
						},
					}},
				},
			},
		},
	}

	authCtx := c.authCtx(ctx)
	stream, err := c.stateClient.GetActiveContracts(authCtx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to query active contracts for party %s: %w", partyID, err)
	}

	activeContracts := make([]*v2.ActiveContract, 0)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading active contracts for party %s: %w", partyID, err)
		}

		contract := resp.GetActiveContract()
		if contract == nil {
			continue
		}
		event := contract.GetCreatedEvent()
		if event == nil {
			continue
		}

		c.logger.
			WithField(KeyContractId, event.GetContractId()).
			Info("found contract")

		activeContracts = append(activeContracts, contract)
	}

	return activeContracts, nil
}

func (c *GrpcLedgerClient) GetTokenHoldingContracts(ctx context.Context, partyID string, ledgerEnd int64, packageID string) ([]*v2.ActiveContract, error) {
	if partyID == "" {
		return nil, errors.New("empty required argument: partyID")
	}
	if packageID == "" {
		return nil, errors.New("empty required argument: packageID")
	}

	req := &v2.GetActiveContractsRequest{
		ActiveAtOffset: ledgerEnd,
		EventFormat: &v2.EventFormat{
			Verbose: true,
			FiltersByParty: map[string]*v2.Filters{
				partyID: {
					Cumulative: []*v2.CumulativeFilter{{
						IdentifierFilter: &v2.CumulativeFilter_InterfaceFilter{
							InterfaceFilter: &v2.InterfaceFilter{
								InterfaceId: &v2.Identifier{
									PackageId:  packageID,
									ModuleName: "Splice.Api.Token.HoldingV1",
									EntityName: "Holding",
								},
								IncludeInterfaceView:    true,
								IncludeCreatedEventBlob: false,
							},
						},
					}},
				},
			},
		},
	}

	authCtx := c.authCtx(ctx)
	stream, err := c.stateClient.GetActiveContracts(authCtx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to query token holding contracts for party %s: %w", partyID, err)
	}

	activeContracts := make([]*v2.ActiveContract, 0)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading token holding contracts for party %s: %w", partyID, err)
		}
		contract := resp.GetActiveContract()
		if contract == nil || contract.GetCreatedEvent() == nil {
			continue
		}
		activeContracts = append(activeContracts, contract)
	}
	return activeContracts, nil
}

func (c *GrpcLedgerClient) GetTokenTransferFactoryContracts(ctx context.Context, partyID string, ledgerEnd int64, packageID string) ([]*v2.ActiveContract, error) {
	if partyID == "" {
		return nil, errors.New("empty required argument: partyID")
	}
	if packageID == "" {
		return nil, errors.New("empty required argument: packageID")
	}

	req := &v2.GetActiveContractsRequest{
		ActiveAtOffset: ledgerEnd,
		EventFormat: &v2.EventFormat{
			Verbose: true,
			FiltersByParty: map[string]*v2.Filters{
				partyID: {
					Cumulative: []*v2.CumulativeFilter{{
						IdentifierFilter: &v2.CumulativeFilter_InterfaceFilter{
							InterfaceFilter: &v2.InterfaceFilter{
								InterfaceId: &v2.Identifier{
									PackageId:  packageID,
									ModuleName: "Splice.Api.Token.TransferInstructionV1",
									EntityName: "TransferFactory",
								},
								IncludeInterfaceView:    true,
								IncludeCreatedEventBlob: true,
							},
						},
					}},
				},
			},
		},
	}

	authCtx := c.authCtx(ctx)
	stream, err := c.stateClient.GetActiveContracts(authCtx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to query token transfer factory contracts for party %s: %w", partyID, err)
	}

	activeContracts := make([]*v2.ActiveContract, 0)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading token transfer factory contracts for party %s: %w", partyID, err)
		}
		contract := resp.GetActiveContract()
		if contract == nil || contract.GetCreatedEvent() == nil {
			continue
		}
		activeContracts = append(activeContracts, contract)
	}
	return activeContracts, nil
}

func (c *GrpcLedgerClient) CreateUser(ctx context.Context, partyId string) error {
	authCtx := c.authCtx(ctx)
	req := &admin.GrantUserRightsRequest{
		UserId: c.validatorServiceUserID,
		Rights: []*admin.Right{
			{
				Kind: &admin.Right_CanReadAs_{
					CanReadAs: &admin.Right_CanReadAs{
						Party: partyId,
					},
				},
			},
			{
				Kind: &admin.Right_CanActAs_{
					CanActAs: &admin.Right_CanActAs{
						Party: partyId,
					},
				},
			},
		},
	}

	_, err := c.userManagementClient.GrantUserRights(authCtx, req)
	if isAlreadyExists(err) {
		c.logger.WithFields(logrus.Fields{
			KeyMethod: "CreateUser",
		}).Warn("user already exists")
		return nil
	} else {
		return err
	}
}

// CreateExternalPartySetupProposal calls the configured validator REST API to create
// an ExternalPartySetupProposal for the given party.
func (c *GrpcLedgerClient) CreateExternalPartySetupProposal(ctx context.Context, partyID string) error {
	body, err := json.Marshal(map[string]string{
		"user_party_id": partyID,
	})
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	endpoint := "/api/validator/v0/admin/external-party/setup-proposal"
	logger := c.logger.WithFields(logrus.Fields{
		KeyMethod:  "POST",
		"endpoint": endpoint,
	})
	logger.Trace("request")
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.restAPIURL+endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bz, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("creating ExternalPartySetupProposal for %s: status %d", partyID, resp.StatusCode)
		}
		errBody := string(bz)
		if strings.Contains(errBody, "ExternalPartySetupProposal contract already exists") ||
			strings.Contains(errBody, "TransferPreapproval contract already exists") {
			return nil
		} else {
			return fmt.Errorf("creating ExternalPartySetupProposal for %s(%d): %s", partyID, resp.StatusCode, string(bz))
		}
	}

	return nil
}

func (c *GrpcLedgerClient) doScanProxyRequest(ctx context.Context, token string, path string, body any, out any) error {
	return c.doScanProxyRequestWithMethod(ctx, token, http.MethodPost, path, body, out)
}

func (c *GrpcLedgerClient) doScanProxyRequestWithMethod(ctx context.Context, token string, method string, path string, body any, out any) error {
	requestBody := []byte("{}")
	var err error
	if body != nil {
		requestBody, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal scan proxy inner body: %w", err)
		}
	}
	targetURL := strings.TrimRight(c.scanAPIURL, "/") + path
	envelope := scanProxyRequest{
		Method: method,
		URL:    targetURL,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(requestBody),
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal scan proxy request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.scanProxyURL,
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}

type AmuletRulesContract struct {
	TemplateID       string `json:"template_id"`
	ContractID       string `json:"contract_id"`
	CreatedEventBlob []byte `json:"created_event_blob"`
	Payload          struct {
		DSO string `json:"dso"`
	} `json:"payload"`
}

type AmuletRules struct {
	AmuletRulesUpdate struct {
		Contract AmuletRulesContract `json:"contract"`
		DomainID string              `json:"domain_id"`
	} `json:"amulet_rules_update"`
}

func (a AmuletRules) GetSpliceId() string {
	return strings.SplitN(a.AmuletRulesUpdate.Contract.TemplateID, ":", 2)[0]
}

// Make sure to auth with canton-ui token
func (c *GrpcLedgerClient) GetAmuletRules(ctx context.Context, token string) (*AmuletRules, error) {
	var result AmuletRules
	if err := c.doScanProxyRequest(ctx, token, "/api/scan/v0/amulet-rules", map[string]any{}, &result); err != nil {
		return nil, fmt.Errorf("fetching amulet rules: %w", err)
	}
	return &result, nil
}

type RoundPayload struct {
	Round struct {
		Number string `json:"number"`
	} `json:"round"`
	OpensAt        string `json:"opensAt"`        // e.g., "2026-03-10T11:42:21.088197Z"
	TargetClosesAt string `json:"targetClosesAt"` // e.g., "2026-03-10T12:02:21.088197Z"
}

type RoundContract struct {
	ContractID       string       `json:"contract_id"`
	TemplateID       string       `json:"template_id"`
	Payload          RoundPayload `json:"payload"`
	CreatedEventBlob []byte       `json:"created_event_blob"`
}

type RoundEntry struct {
	Contract RoundContract `json:"contract"`
	DomainID string        `json:"domain_id"`
}

type OpenAndIssuingMiningRounds struct {
	OpenMiningRounds    map[string]RoundEntry `json:"open_mining_rounds"`
	IssuingMiningRounds map[string]RoundEntry `json:"issuing_mining_rounds"`
}

// GetLatestOpenMiningRound returns the open mining round with the highest round number.
func (r *OpenAndIssuingMiningRounds) GetLatestOpenMiningRound() (*RoundEntry, error) {
	now := time.Now().UTC() // ledger time; adjust if simulating
	var current *RoundEntry
	var highestRound int64 = -1

	for _, entry := range r.OpenMiningRounds {
		// parse round number
		n, err := strconv.ParseInt(entry.Contract.Payload.Round.Number, 10, 64)
		if err != nil {
			continue
		}

		// parse opensAt
		opensAt, err := time.Parse(time.RFC3339Nano, entry.Contract.Payload.OpensAt)
		if err != nil {
			continue
		}

		// parse targetClosesAt
		targetClosesAt, err := time.Parse(time.RFC3339Nano, entry.Contract.Payload.TargetClosesAt)
		if err != nil {
			continue
		}

		if !now.Before(opensAt) && !now.After(targetClosesAt) {
			// round is currently open
			if n > highestRound {
				highestRound = n
				e := entry
				current = &e
			}
		}
	}

	if current == nil {
		return nil, fmt.Errorf("no currently open mining rounds found")
	}
	return current, nil
}

// GetLatestOpenMiningRound returns the open mining round with the highest round number.
func (r *OpenAndIssuingMiningRounds) GetLatestIssuingMiningRound() (*RoundEntry, error) {
	now := time.Now().UTC()

	for _, entry := range r.IssuingMiningRounds {
		opensAt, err := time.Parse(time.RFC3339Nano, entry.Contract.Payload.OpensAt)
		if err != nil {
			continue
		}
		targetClosesAt, err := time.Parse(time.RFC3339Nano, entry.Contract.Payload.TargetClosesAt)
		if err != nil {
			continue
		}

		// Round is currently open
		if !now.Before(opensAt) && now.Before(targetClosesAt) {
			return &entry, nil
		}
	}

	return nil, fmt.Errorf("no currently open issuing mining round found")
}

func (c *GrpcLedgerClient) GetOpenAndIssuingMiningRound(ctx context.Context, token string) (*RoundEntry, *RoundEntry, error) {
	var result OpenAndIssuingMiningRounds
	body := map[string]any{
		"cached_open_mining_round_contract_ids": []string{},
		"cached_issuing_round_contract_ids":     []string{},
	}
	if err := c.doScanProxyRequest(ctx, token, "/api/scan/v0/open-and-issuing-mining-rounds", body, &result); err != nil {
		return nil, nil, fmt.Errorf("fetching mining rounds: %w", err)
	}

	latestOpenRound, err := result.GetLatestOpenMiningRound()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get lastest open mining round: %w", err)
	}

	latestIssuingRound, err := result.GetLatestIssuingMiningRound()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get lastest issuing mining round: %w", err)
	}
	return latestOpenRound, latestIssuingRound, nil
}

func (c *GrpcLedgerClient) GetTokenTransferFactory(
	ctx context.Context,
	token string,
	choiceArguments map[string]any,
) (*TokenTransferFactoryContext, error) {
	body := map[string]any{
		"choiceArguments":    choiceArguments,
		"excludeDebugFields": true,
	}
	var result TokenTransferFactoryContext
	if err := c.doScanProxyRequest(ctx, token, "/registry/transfer-instruction/v1/transfer-factory", body, &result); err != nil {
		return nil, fmt.Errorf("fetching token transfer factory: %w", err)
	}
	if result.FactoryID == "" {
		return nil, errors.New("token transfer factory response missing factoryId")
	}
	return &result, nil
}

func (c *GrpcLedgerClient) GetTokenInstrumentMetadata(
	ctx context.Context,
	token string,
	instrumentID string,
) (*TokenInstrumentMetadata, error) {
	path := "/registry/metadata/v1/instruments/" + url.PathEscape(instrumentID)
	var result TokenInstrumentMetadata
	if err := c.doScanProxyRequestWithMethod(ctx, token, http.MethodGet, path, nil, &result); err != nil {
		return nil, fmt.Errorf("fetching token instrument metadata: %w", err)
	}
	if result.ID == "" {
		return nil, errors.New("token instrument metadata response missing id")
	}
	return &result, nil
}

func (c *GrpcLedgerClient) GetTokenMetadataRegistryInfo(
	ctx context.Context,
	token string,
) (*TokenMetadataRegistryInfo, error) {
	var result TokenMetadataRegistryInfo
	if err := c.doScanProxyRequestWithMethod(ctx, token, http.MethodGet, "/registry/metadata/v1/info", nil, &result); err != nil {
		return nil, fmt.Errorf("fetching token metadata registry info: %w", err)
	}
	if result.AdminID == "" {
		return nil, errors.New("token metadata registry info response missing adminId")
	}
	return &result, nil
}

func decodeCreatedEventBlob(value string) ([]byte, error) {
	if value == "" {
		return nil, errors.New("empty createdEventBlob")
	}
	blob, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode createdEventBlob: %w", err)
	}
	return blob, nil
}

func (c *GrpcLedgerClient) HasTransferPreapprovalContract(ctx context.Context, contracts []*v2.ActiveContract) bool {
	preapprovalContractID := ""
	for _, c := range contracts {
		created := c.GetCreatedEvent()
		if created == nil {
			continue
		}
		tid := created.GetTemplateId()
		if tid == nil || !isPreapprovalTemplate(tid) {
			continue
		}

		preapprovalContractID = created.GetContractId()
	}

	return preapprovalContractID != ""
}

// newRegisterCommandId generates a UUID-style command ID for registration calls.
func newRegisterCommandId() string {
	return cantonproto.NewCommandID()
}

func (c *GrpcLedgerClient) PrepareSubmissionRequest(ctx context.Context, command *v2.Command, commandID string, partyID string, synchronizerID string) (*interactive.PrepareSubmissionResponse, error) {
	prepareReq := cantonproto.NewPrepareRequest(commandID, synchronizerID, []string{partyID}, []string{partyID}, []*v2.Command{command}, nil)
	return c.PrepareSubmission(ctx, prepareReq)
}

func (c *GrpcLedgerClient) AcceptExternalPartySetupProposal(ctx context.Context, partyId string, privateKey ed25519.PrivateKey) error {
	ledgerEnd, err := c.GetLedgerEnd(ctx)
	if err != nil {
		return fmt.Errorf("failed to get ledger end: %w", err)
	}

	contracts, err := c.GetActiveContracts(ctx, partyId, ledgerEnd, true)
	if err != nil {
		return fmt.Errorf("failed to get active contracts: %w", err)
	}

	proposalContractID := ""
	var proposalTemplateID *v2.Identifier
	for _, c := range contracts {
		event := c.GetCreatedEvent()
		if event == nil {
			continue
		}
		tid := event.GetTemplateId()
		if tid.GetEntityName() == "ExternalPartySetupProposal" {
			proposalContractID = event.GetContractId()
			proposalTemplateID = tid
			break
		}
	}

	logger := c.logger.WithField(KeyParty, partyId)
	if proposalContractID == "" {
		logger.WithField(KeyParty, partyId).Debug("no ExternalPartySetupProposal found (may already be accepted)")
		return nil
	}

	cmd := &v2.Command{
		Command: &v2.Command_Exercise{
			Exercise: &v2.ExerciseCommand{
				TemplateId:     proposalTemplateID,
				ContractId:     proposalContractID,
				Choice:         "ExternalPartySetupProposal_Accept",
				ChoiceArgument: &v2.Value{Sum: &v2.Value_Record{Record: &v2.Record{}}},
			},
		},
	}

	commandID := newRegisterCommandId()
	synchronizerID, err := c.ResolveSynchronizerID(ctx, partyId, "")
	if err != nil {
		return fmt.Errorf("failed to resolve synchronizer: %w", err)
	}
	prepareResp, err := c.PrepareSubmissionRequest(ctx, cmd, commandID, partyId, synchronizerID)
	if err != nil {
		return fmt.Errorf("failed to prepare submission for party setup proposal accept: %w", err)
	}

	// Sign the prepared transaction hash with the external party's private key.
	txSig := ed25519.Sign(privateKey, prepareResp.GetPreparedTransactionHash())
	_, keyFingerprint, err := cantonaddress.ParsePartyID(xc.Address(partyId))
	if err != nil {
		return fmt.Errorf("failed to parse PartyID: %w", err)
	}
	executeReq := &interactive.ExecuteSubmissionAndWaitRequest{
		PreparedTransaction: prepareResp.GetPreparedTransaction(),
		PartySignatures: &interactive.PartySignatures{
			Signatures: []*interactive.SinglePartySignatures{
				{
					Party: partyId,
					Signatures: []*v2.Signature{
						{
							Format:               v2.SignatureFormat_SIGNATURE_FORMAT_RAW,
							Signature:            txSig,
							SignedBy:             keyFingerprint,
							SigningAlgorithmSpec: v2.SigningAlgorithmSpec_SIGNING_ALGORITHM_SPEC_ED25519,
						},
					},
				},
			},
		},
		DeduplicationPeriod: &interactive.ExecuteSubmissionAndWaitRequest_DeduplicationDuration{
			DeduplicationDuration: durationpb.New(c.deduplicationWindow),
		},
		SubmissionId:         newRegisterCommandId(),
		HashingSchemeVersion: prepareResp.GetHashingSchemeVersion(),
	}

	logrus.WithField("rpc", "ExecuteSubmissionAndWait").Trace("ExternalPartySetupProposal_Accept")
	_, err = c.ExecuteSubmissionAndWait(ctx, executeReq)
	if err != nil {
		if isAlreadyExists(err) {
			logrus.WithField("party_id", partyId).Debug("canton: setup proposal already accepted")
			return nil
		}
		return fmt.Errorf("ExecuteSubmissionAndWait: %w", err)
	}
	return nil
}

func (c *GrpcLedgerClient) CompleteAcceptedTransferOffer(
	ctx context.Context,
	senderPartyID string,
	amuletRules *AmuletRules,
	openMiningRound *RoundEntry,
	issuingMiningRound *RoundEntry,
	privateKey ed25519.PrivateKey,
	amulets []*v2.ActiveContract,
) error {
	var acceptedOfferContractID string
	var acceptedOfferTemplateID *v2.Identifier
	for _, ac := range amulets {
		event := ac.GetCreatedEvent()
		if event == nil {
			continue
		}
		tid := event.GetTemplateId()
		if tid.GetModuleName() == "Splice.Wallet.TransferOffer" && tid.GetEntityName() == "AcceptedTransferOffer" {
			acceptedOfferContractID = event.GetContractId()
			acceptedOfferTemplateID = tid
			break
		}
	}
	if acceptedOfferContractID == "" {
		c.logger.WithField(KeyParty, senderPartyID).Debug("no AcceptedTransferOffer found, skipping completion")
		return nil
	}

	amuletRulesID := amuletRules.AmuletRulesUpdate.Contract.ContractID
	openMiningRoundID := openMiningRound.Contract.ContractID

	// Build TransferInput list from amulet contracts, excluding the AcceptedTransferOffer itself.
	transferInputs := make([]*v2.Value, 0, len(amulets))
	for _, ac := range amulets {
		event := ac.GetCreatedEvent()
		if event == nil {
			continue
		}
		if event.GetContractId() == acceptedOfferContractID {
			continue
		}
		entityName := event.GetTemplateId().GetEntityName()
		if entityName != "Amulet" {
			// skip non-Amulet contracts
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

	c.logger.
		WithField("contract_id", openMiningRound.Contract.ContractID).
		Info("opening contract")

	c.logger.
		WithField("contract_id", issuingMiningRound.Contract.ContractID).
		Info("issuing contract")

	// Build disclosed contracts: all amulets (including AcceptedTransferOffer) +
	// amulet rules + open mining round.
	disclosedContracts := make([]*v2.DisclosedContract, 0, len(amulets)+2)
	for _, ac := range amulets {
		event := ac.GetCreatedEvent()
		if event == nil {
			continue
		}
		entityName := event.GetTemplateId().GetEntityName()
		if entityName != "Amulet" {
			// skip non-Amulet contracts
			continue
		}

		c.logger.
			WithField("contract_entity", event.GetTemplateId().GetEntityName()).
			Info("disclosing contract")

		disclosedContracts = append(disclosedContracts, &v2.DisclosedContract{
			TemplateId:       event.GetTemplateId(),
			ContractId:       event.GetContractId(),
			CreatedEventBlob: event.GetCreatedEventBlob(),
		})
	}

	// Disclose AmuletRules: parse "packageId:ModuleName:EntityName" template ID string.
	amuletRulesTemplateParts := strings.SplitN(amuletRules.AmuletRulesUpdate.Contract.TemplateID, ":", 3)
	if len(amuletRulesTemplateParts) == 3 {
		c.logger.
			WithField("contract_entity", amuletRulesTemplateParts[2]).
			Info("disclosing contract")
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

	c.logger.
		WithField("contract_id", openMiningRound.Contract.ContractID).
		WithField("template", openMiningRound.Contract.TemplateID).
		WithField("blob", openMiningRound.Contract.CreatedEventBlob).
		Info("disclosing contract")

	openParts := strings.Split(openMiningRound.Contract.TemplateID, ":")
	// Disclose OpenMiningRound (no template ID available from scan API response).
	disclosedContracts = append(disclosedContracts, &v2.DisclosedContract{
		TemplateId: &v2.Identifier{
			PackageId:  openParts[0],
			ModuleName: openParts[1],
			EntityName: openParts[2],
		},
		ContractId:       openMiningRoundID,
		CreatedEventBlob: openMiningRound.Contract.CreatedEventBlob,
	})

	c.logger.
		WithField("template", issuingMiningRound.Contract.TemplateID).
		WithField("contract_id", issuingMiningRound.Contract.ContractID).
		WithField("blob", issuingMiningRound.Contract.CreatedEventBlob).
		Info("disclosing contract")

	issuingParts := strings.Split(issuingMiningRound.Contract.TemplateID, ":")
	// Disclose OpenMiningRound (no template ID available from scan API response).
	disclosedContracts = append(disclosedContracts, &v2.DisclosedContract{
		TemplateId: &v2.Identifier{
			PackageId:  issuingParts[0],
			ModuleName: issuingParts[1],
			EntityName: issuingParts[2],
		},
		ContractId:       issuingMiningRound.Contract.ContractID,
		CreatedEventBlob: issuingMiningRound.Contract.CreatedEventBlob,
	})

	rn, err := strconv.Atoi(issuingMiningRound.Contract.Payload.Round.Number)
	if err != nil {
		panic(err)
	}
	// Build GenMap for issuingMiningRounds
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
											Value: &v2.Value{
												Sum: &v2.Value_Int64{
													Int64: int64(rn),
												},
											},
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

	cmd := &v2.Command{
		Command: &v2.Command_Exercise{
			Exercise: &v2.ExerciseCommand{
				TemplateId: acceptedOfferTemplateID,
				ContractId: acceptedOfferContractID,
				Choice:     "AcceptedTransferOffer_Complete",
				ChoiceArgument: &v2.Value{
					Sum: &v2.Value_Record{
						Record: &v2.Record{
							Fields: []*v2.RecordField{
								{
									Label: "inputs",
									Value: &v2.Value{
										Sum: &v2.Value_List{
											List: &v2.List{
												Elements: transferInputs,
											},
										},
									},
								},
								{
									Label: "transferContext",
									Value: &v2.Value{
										Sum: &v2.Value_Record{
											Record: &v2.Record{
												Fields: []*v2.RecordField{
													{
														Label: "amuletRules",
														Value: &v2.Value{
															Sum: &v2.Value_ContractId{
																ContractId: amuletRulesID,
															},
														},
													},
													{
														Label: "context",
														Value: &v2.Value{
															Sum: &v2.Value_Record{
																Record: &v2.Record{
																	Fields: []*v2.RecordField{
																		{
																			Label: "openMiningRound",
																			Value: &v2.Value{
																				Sum: &v2.Value_ContractId{
																					ContractId: openMiningRoundID,
																				},
																			},
																		},
																		{
																			Label: "issuingMiningRounds",
																			Value: issuingMiningRounds,
																		},
																		{
																			Label: "validatorRights",
																			Value: &v2.Value{
																				Sum: &v2.Value_GenMap{
																					GenMap: &v2.GenMap{
																						Entries: []*v2.GenMap_Entry{}, // empty for now
																					},
																				},
																			},
																		},
																		{
																			Label: "featuredAppRight",
																			Value: &v2.Value{
																				Sum: &v2.Value_Optional{
																					Optional: &v2.Optional{},
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
								{
									Label: "walletProvider",
									Value: &v2.Value{
										Sum: &v2.Value_Party{
											Party: senderPartyID,
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

	prepareReq := &interactive.PrepareSubmissionRequest{
		UserId:    c.validatorServiceUserID,
		CommandId: newRegisterCommandId(),
		Commands:  []*v2.Command{cmd},
		// ActAs:     []string{senderPartyID, ValidatorPartyId},
		ReadAs: []string{senderPartyID, c.validatorPartyID},
		ActAs:  []string{senderPartyID},
		// ReadAs:             []string{senderPartyID},
		SynchronizerId:     amuletRules.AmuletRulesUpdate.DomainID,
		DisclosedContracts: disclosedContracts,
		VerboseHashing:     false,
	}

	prepareResp, err := c.PrepareSubmission(ctx, prepareReq)
	if err != nil {
		return fmt.Errorf("preparing AcceptedTransferOffer_Complete: %w", err)
	}

	txSig := ed25519.Sign(privateKey, prepareResp.GetPreparedTransactionHash())
	_, keyFingerprint, err := cantonaddress.ParsePartyID(xc.Address(senderPartyID))
	if err != nil {
		return fmt.Errorf("parsing sender party ID: %w", err)
	}

	executeReq := &interactive.ExecuteSubmissionAndWaitRequest{
		PreparedTransaction: prepareResp.GetPreparedTransaction(),
		PartySignatures: &interactive.PartySignatures{
			Signatures: []*interactive.SinglePartySignatures{
				{
					Party: senderPartyID,
					Signatures: []*v2.Signature{
						{
							Format:               v2.SignatureFormat_SIGNATURE_FORMAT_RAW,
							Signature:            txSig,
							SignedBy:             keyFingerprint,
							SigningAlgorithmSpec: v2.SigningAlgorithmSpec_SIGNING_ALGORITHM_SPEC_ED25519,
						},
					},
				},
			},
		},
		DeduplicationPeriod: &interactive.ExecuteSubmissionAndWaitRequest_DeduplicationDuration{
			DeduplicationDuration: durationpb.New(c.deduplicationWindow),
		},
		SubmissionId:         newRegisterCommandId(),
		HashingSchemeVersion: prepareResp.GetHashingSchemeVersion(),
	}

	_, err = c.ExecuteSubmissionAndWait(ctx, executeReq)
	if err != nil {
		return fmt.Errorf("executing AcceptedTransferOffer_Complete: %w", err)
	}

	return nil
}

// GetUpdateById fetches a transaction (update) by its updateId from the Canton ledger,
// scoped to a specific party to avoid requiring super-reader permissions.
func (c *GrpcLedgerClient) GetUpdateById(ctx context.Context, partyID string, updateId string) (*v2.GetUpdateResponse, error) {
	if partyID == "" {
		return nil, errors.New("empty party id")
	}
	authCtx := c.authCtx(ctx)
	req := &v2.GetUpdateByIdRequest{
		UpdateId: updateId,
		UpdateFormat: &v2.UpdateFormat{
			IncludeTransactions: &v2.TransactionFormat{
				TransactionShape: v2.TransactionShape_TRANSACTION_SHAPE_LEDGER_EFFECTS,
				EventFormat: &v2.EventFormat{
					FiltersByParty: map[string]*v2.Filters{
						partyID: {
							Cumulative: []*v2.CumulativeFilter{{
								IdentifierFilter: &v2.CumulativeFilter_WildcardFilter{
									WildcardFilter: &v2.WildcardFilter{},
								},
							}},
						},
					},
					Verbose: true,
				},
			},
		},
	}
	resp, err := c.updateClient.GetUpdateById(authCtx, req)
	if err != nil {
		return nil, fmt.Errorf("GetUpdateById(%s, %s): %w", partyID, updateId, err)
	}
	return resp, nil
}

func (c *GrpcLedgerClient) RecoverUpdateIdBySubmissionId(ctx context.Context, beginExclusive int64, partyID string, submissionId string) (string, error) {
	if partyID == "" {
		return "", errors.New("empty party id")
	}
	if submissionId == "" {
		return "", errors.New("empty submission id")
	}

	upperBound, err := c.GetLedgerEnd(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get ledger end for recovery: %w", err)
	}
	if upperBound <= beginExclusive {
		return "", fmt.Errorf("no ledger updates after offset %d", beginExclusive)
	}

	streamCtx, cancel := context.WithTimeout(c.authCtx(ctx), 15*time.Second)
	defer cancel()

	stream, err := c.completionClient.CompletionStream(streamCtx, &v2.CompletionStreamRequest{
		UserId:         c.validatorServiceUserID,
		Parties:        []string{partyID},
		BeginExclusive: beginExclusive,
	})
	if err != nil {
		return "", fmt.Errorf("CompletionStream(%d): %w", beginExclusive, err)
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("completion stream recv: %w", err)
		}

		if completion := resp.GetCompletion(); completion != nil {
			if completion.GetSubmissionId() == submissionId && completion.GetUpdateId() != "" {
				return completion.GetUpdateId(), nil
			}
			if completion.GetOffset() >= upperBound {
				break
			}
			continue
		}

		if checkpoint := resp.GetOffsetCheckpoint(); checkpoint != nil && checkpoint.GetOffset() >= upperBound {
			break
		}
	}

	return "", fmt.Errorf("could not recover update id for submission %q in offsets (%d, %d]", submissionId, beginExclusive, upperBound)
}

func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "ALREADY_EXISTS") ||
		strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "AlreadyExists")
}

func (c *GrpcLedgerClient) ExecuteTransferInstructionSend(
	ctx context.Context,
	contractID string,
	senderParty string,
	privKey ed25519.PrivateKey,
) error {
	// 1. Prepare
	prepareReq := &interactive.PrepareSubmissionRequest{
		CommandId: "transfer-send-" + contractID,
		ActAs:     []string{senderParty},
		Commands: []*v2.Command{
			{
				Command: &v2.Command_Exercise{
					Exercise: &v2.ExerciseCommand{
						TemplateId: &v2.Identifier{
							PackageId:  "splice-amulet", // resolved by canton
							ModuleName: "Splice.AmuletAllocation.TransferInstruction",
							EntityName: "TransferInstruction",
						},
						ContractId: contractID,
						Choice:     "TransferInstruction_Send",
						ChoiceArgument: &v2.Value{
							Sum: &v2.Value_Record{
								Record: &v2.Record{
									// TransferInstruction_Send takes no fields
									Fields: []*v2.RecordField{},
								},
							},
						},
					},
				},
			},
		},
		DisclosedContracts: []*v2.DisclosedContract{},
	}

	prepareResp, err := c.PrepareSubmission(ctx, prepareReq)
	if err != nil {
		return fmt.Errorf("prepare failed: %w", err)
	}

	prepared := prepareResp.GetPreparedTransaction()
	if prepared == nil {
		return fmt.Errorf("no prepared transaction returned")
	}

	txHash := prepareResp.GetPreparedTransactionHash()

	// 2. Sign
	signature := ed25519.Sign(privKey, txHash)

	// 3. Submit
	submitReq := &interactive.ExecuteSubmissionAndWaitRequest{
		PreparedTransaction: prepared,
		PartySignatures: &interactive.PartySignatures{
			Signatures: []*interactive.SinglePartySignatures{
				{
					Party: "",
					Signatures: []*v2.Signature{
						{
							Format:    v2.SignatureFormat_SIGNATURE_FORMAT_RAW,
							Signature: signature,
							SignedBy:  hex.EncodeToString(privKey.Public().(ed25519.PublicKey)),
						},
					},
				},
			},
		},
	}

	_, err = c.ExecuteSubmissionAndWait(ctx, submitReq)
	if err != nil {
		return fmt.Errorf("submit failed: %w", err)
	}

	return nil
}

// isAmuletTemplate returns true if the identifier refers to a Splice Amulet contract.
// We match on module+entity name only; package IDs are deployment-specific.
func isPreapprovalTemplate(id *v2.Identifier) bool {
	return id.GetModuleName() == "Splice.AmuletRules" &&
		id.GetEntityName() == "TransferPreapproval"
}

// isAmuletTemplate returns true if the identifier refers to a Splice Amulet contract.
// We match on module+entity name only; package IDs are deployment-specific.
func isAmuletTemplate(id *v2.Identifier) bool {
	return id.GetModuleName() == "Splice.Amulet" &&
		(id.GetEntityName() == "Amulet" || id.GetEntityName() == "LockedAmulet")
}

// ExtractAmuletBalance extracts and converts the initialAmount from an Amulet contract's
// createArguments Record. The Splice.Amulet / Splice.Fees.ExpiringAmount schema is:
//
//	Amulet { dso: Party, owner: Party, amount: ExpiringAmount }
//	ExpiringAmount { initialAmount: Decimal, createdAt: Round, ratePerRound: RatePerRound }
//
// initialAmount is a Daml Decimal – it arrives as a proto Value_Numeric string (e.g. "100.5000000000").
// We parse it as a human-readable amount and convert to blockchain units using the chain's decimal places.
func ExtractAmuletBalance(record *v2.Record, decimals int32) (xc.AmountBlockchain, bool) {
	if record == nil {
		return xc.AmountBlockchain{}, false
	}
	// Walk: Amulet.amount (ExpiringAmount record)
	for _, field := range record.GetFields() {
		if field.GetLabel() != LabelAmount {
			continue
		}
		expiringAmount := field.GetValue().GetRecord()
		if expiringAmount == nil {
			continue
		}
		// Walk: ExpiringAmount.initialAmount (Numeric directly)
		for _, af := range expiringAmount.GetFields() {
			if af.GetLabel() != LabelInitialAmount {
				continue
			}
			numeric := af.GetValue().GetNumeric()
			if numeric == "" {
				continue
			}
			human, err := xc.NewAmountHumanReadableFromStr(numeric)
			if err != nil {
				continue
			}
			return human.ToBlockchain(decimals), true
		}
	}
	return xc.AmountBlockchain{}, false
}

// NewCommandId generates a UUID-style unique command ID
func NewCommandId() string {
	return cantonproto.NewCommandID()
}
