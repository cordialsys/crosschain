package client

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	xccall "github.com/cordialsys/crosschain/call"
	cantonaddress "github.com/cordialsys/crosschain/chain/canton/address"
	cantonkc "github.com/cordialsys/crosschain/chain/canton/keycloak"
	cantontx "github.com/cordialsys/crosschain/chain/canton/tx"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	cantonproto "github.com/cordialsys/crosschain/chain/canton/types"
	v2 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/admin"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	xclient "github.com/cordialsys/crosschain/client"
	cantonclientconfig "github.com/cordialsys/crosschain/client/canton"
	xcerrors "github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// Client for Canton using the gRPC Ledger API
type Client struct {
	Asset *xc.ChainConfig

	LedgerClient *GrpcLedgerClient

	// ValidatorKC fetches validator-level tokens (client_credentials grant).
	ValidatorKC *cantonkc.Client
	// CantonUiKC acquires canton-ui tokens for scan proxy HTTP calls.
	CantonUiKC *cantonkc.Client

	CantonUiUsername string
	CantonUiPassword string
	CantonCfg        *CantonConfig
}

var _ xclient.Client = &Client{}
var _ xclient.CallClient = &Client{}
var _ xclient.OfferClient = &Client{}

func (client *Client) logger() *logrus.Entry {
	if client != nil && client.LedgerClient != nil && client.LedgerClient.Logger != nil {
		return client.LedgerClient.Logger
	}
	return logrus.NewEntry(logrus.StandardLogger())
}

func ValidatorServiceUserIDFromToken(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", errors.New("invalid validator auth token")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode validator auth token payload: %w", err)
	}

	var claims struct {
		PreferredUsername string `json:"preferred_username"`
		Subject           string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("decode validator auth token claims: %w", err)
	}
	if claims.PreferredUsername == "" {
		if claims.Subject == "" {
			return "", errors.New("validator auth token missing preferred_username and sub")
		}
		return claims.Subject, nil
	}
	return claims.PreferredUsername, nil
}

func disclosedContractIDs(disclosedContracts []*v2.DisclosedContract) []string {
	ids := make([]string, 0, len(disclosedContracts))
	for _, disclosed := range disclosedContracts {
		if disclosed == nil || disclosed.GetContractId() == "" {
			continue
		}
		ids = append(ids, disclosed.GetContractId())
	}
	return ids
}

func fetchValidatorPartyID(ctx context.Context, RestAPIURL string) (string, error) {
	endpoint := strings.TrimRight(RestAPIURL, "/") + "/api/validator/v0/validator-user"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create validator user request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch validator user: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch validator user returned %d: %s", resp.StatusCode, body)
	}

	var payload struct {
		PartyID string `json:"party_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("decode validator user response: %w", err)
	}
	if payload.PartyID == "" {
		return "", errors.New("validator user response missing party_id")
	}

	return payload.PartyID, nil
}

// NewClient returns a new Canton gRPC Client
func NewClient(cfgI *xc.ChainConfig) (*Client, error) {
	cfg := cfgI.GetChain()

	if cfg.URL == "" {
		return nil, fmt.Errorf("no URL configured for Canton client")
	}

	CantonCfg := cfgI.CantonConfig
	if err := CantonCfg.Validate(); err != nil {
		return nil, err
	}
	if CantonCfg.KeycloakURL == "" {
		return nil, fmt.Errorf("missing canton config field keycloak_url")
	}
	if CantonCfg.KeycloakRealm == "" {
		return nil, fmt.Errorf("missing canton config field keycloak_realm")
	}
	if CantonCfg.RestAPIURL == "" {
		return nil, fmt.Errorf("missing canton config field rest_api_url")
	}
	if CantonCfg.ScanProxyURL == "" {
		return nil, fmt.Errorf("missing canton config field scan_proxy_url")
	}
	if CantonCfg.ScanAPIURL == "" {
		return nil, fmt.Errorf("missing canton config field scan_api_url")
	}
	if CantonCfg.ValidatorClientID == "" {
		return nil, fmt.Errorf("missing canton config field validator_client_id")
	}

	validatorClientSecret, err := CantonCfg.ValidatorClientSecret.LoadNonEmpty()
	if err != nil {
		return nil, fmt.Errorf("failed to load canton validator auth: %w", err)
	}
	validatorClientID := CantonCfg.ValidatorClientID

	ValidatorPartyID, err := fetchValidatorPartyID(context.Background(), CantonCfg.RestAPIURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch canton validator party id: %w", err)
	}

	catalystPassword, err := CantonCfg.CatalystPassword.LoadNonEmpty()
	if err != nil {
		return nil, fmt.Errorf("failed to load canton ui auth: %w", err)
	}

	client := &Client{
		Asset:            cfgI,
		ValidatorKC:      cantonkc.NewClient(CantonCfg.KeycloakURL, CantonCfg.KeycloakRealm, validatorClientID, validatorClientSecret, ValidatorPartyID),
		CantonUiKC:       cantonkc.NewClient(CantonCfg.KeycloakURL, CantonCfg.KeycloakRealm, validatorClientID, validatorClientSecret, ValidatorPartyID),
		CantonUiUsername: CantonCfg.CatalystUsername,
		CantonUiPassword: catalystPassword,
		CantonCfg:        CantonCfg,
	}

	AuthToken, err := client.ValidatorKC.AdminToken(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch auth token: %w", err)
	}
	if AuthToken == "" {
		return nil, errors.New("invalid AuthToken")
	}
	ValidatorServiceUserID, err := ValidatorServiceUserIDFromToken(AuthToken)
	if err != nil {
		return nil, fmt.Errorf("failed to derive validator service user id from token: %w", err)
	}

	grpcClient, err := NewGrpcLedgerClient(cfg.URL, AuthToken, runtimeIdentityConfig{
		ValidatorPartyID:       ValidatorPartyID,
		ValidatorServiceUserID: ValidatorServiceUserID,
		DeduplicationWindow:    cfgI.TransactionActiveTime,
		RestAPIURL:             CantonCfg.RestAPIURL,
		ScanProxyURL:           CantonCfg.ScanProxyURL,
		ScanAPIURL:             CantonCfg.ScanAPIURL,
		LighthouseAPIURL:       CantonCfg.LighthouseAPIURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GrpcLedgerClient: %w", err)
	}
	client.LedgerClient = grpcClient

	return client, nil
}

// cantonUIToken acquires a canton-ui Keycloak token used for scan proxy HTTP calls.
func (client *Client) cantonUIToken(ctx context.Context) (string, error) {
	if client.CantonUiKC == nil {
		return "", errors.New("canton-ui auth client is not configured")
	}
	resp, err := client.CantonUiKC.AcquireCantonUiToken(ctx, client.CantonUiUsername, client.CantonUiPassword)
	if err != nil {
		return "", fmt.Errorf("failed to acquire canton-ui token: %w", err)
	}
	return resp.AccessToken, nil
}

func (client *Client) resolveTokenRegistryBaseURL(ctx context.Context, contract xc.ContractAddress, InstrumentAdmin string) (string, string, error) {
	if contract == "" {
		return "", "", errors.New("empty token contract")
	}
	if client != nil && client.CantonCfg != nil {
		if baseURL := strings.TrimRight(client.CantonCfg.TokenRegistryURLs[tokenRegistryKey(contract)], "/"); baseURL != "" {
			if client.LedgerClient != nil && strings.TrimRight(baseURL, "/") == strings.TrimRight(client.LedgerClient.ScanAPIURL, "/") && client.LedgerClient.ScanProxyURL != "" {
				uiToken, err := client.cantonUIToken(ctx)
				if err != nil {
					return "", "", fmt.Errorf("failed to acquire canton-ui token for configured scan registry %q: %w", contract, err)
				}
				return baseURL, uiToken, nil
			}
			return baseURL, "", nil
		}
	}

	if client != nil && client.LedgerClient != nil && client.LedgerClient.ScanAPIURL != "" && client.LedgerClient.ScanProxyURL != "" {
		uiToken, err := client.cantonUIToken(ctx)
		if err == nil {
			registryInfo, infoErr := client.LedgerClient.GetTokenMetadataRegistryInfo(ctx, uiToken)
			if infoErr == nil && registryInfo.AdminID == InstrumentAdmin {
				return strings.TrimRight(client.LedgerClient.ScanAPIURL, "/"), uiToken, nil
			}
		}
	}

	return "", "", fmt.Errorf("no token registry configured for contract %q", contract)
}

func (client *Client) resolveSynchronizerID(ctx context.Context, partyID string, fallback string) (string, error) {
	return client.LedgerClient.ResolveSynchronizerID(ctx, partyID, fallback)
}

func (client *Client) resolveValidatorSynchronizerID(ctx context.Context) (string, error) {
	synchronizerID, err := client.resolveSynchronizerID(ctx, client.LedgerClient.ValidatorPartyID, "")
	if err == nil {
		return synchronizerID, nil
	}

	uiToken, tokenErr := client.cantonUIToken(ctx)
	if tokenErr != nil {
		return "", fmt.Errorf("failed to resolve validator synchronizer via validator party (%w) and could not fetch UI token for fallback (%v)", err, tokenErr)
	}
	amuletRules, rulesErr := client.LedgerClient.GetAmuletRules(ctx, uiToken)
	if rulesErr != nil {
		return "", fmt.Errorf("failed to resolve validator synchronizer via validator party (%w) and could not fetch amulet rules fallback (%v)", err, rulesErr)
	}
	return client.resolveSynchronizerID(ctx, "", amuletRules.AmuletRulesUpdate.DomainID)
}

func (client *Client) PrepareTransferOfferCommand(ctx context.Context, args xcbuilder.TransferArgs, amuletRules AmuletRules) (*interactive.PrepareSubmissionResponse, error) {
	commandID := cantonproto.NewCommandID()
	walletPackageID, err := client.LedgerClient.ResolvePackageIDByName(ctx, "splice-wallet")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve splice-wallet package: %w", err)
	}
	cmd := BuildTransferOfferCreateCommand(args, amuletRules, walletPackageID, commandID, client.Asset.GetChain().Decimals)
	synchronizerID, err := client.resolveSynchronizerID(ctx, string(args.GetFrom()), amuletRules.AmuletRulesUpdate.DomainID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve transfer synchronizer: %w", err)
	}

	prepareResp, err := client.LedgerClient.PrepareSubmissionRequest(ctx, cmd, commandID, string(args.GetFrom()), synchronizerID)
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
	cmd, disclosedContracts, err := BuildTransferPreapprovalExerciseCommand(args, amuletRules, openMiningRound, issuingMiningRound, senderContracts, recipientContracts, client.Asset.GetChain().Decimals)
	if err != nil {
		return nil, err
	}
	commandID := cantonproto.NewCommandID()
	synchronizerID, err := client.resolveSynchronizerID(ctx, senderPartyID, amuletRules.AmuletRulesUpdate.DomainID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve transfer synchronizer: %w", err)
	}
	prepareReq := cantonproto.NewPrepareRequest(commandID, synchronizerID, []string{senderPartyID}, []string{senderPartyID, client.LedgerClient.ValidatorPartyID}, []*v2.Command{cmd}, disclosedContracts)

	prepareResp, err := client.LedgerClient.PrepareSubmission(ctx, prepareReq)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare TransferPreapproval_Send: %w", err)
	}

	return prepareResp, nil
}

func (client *Client) PrepareTokenTransferCommand(
	ctx context.Context,
	args xcbuilder.TransferArgs,
	senderContracts []*v2.ActiveContract,
	senderHoldings []*v2.ActiveContract,
	decimals int32,
) (*interactive.PrepareSubmissionResponse, error) {
	transferPackageID, err := client.LedgerClient.ResolvePackageIDByName(ctx, "splice-api-token-transfer-instruction-v1")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve token transfer interface package: %w", err)
	}

	contract, ok := args.GetContract()
	if !ok {
		return nil, fmt.Errorf("missing token contract")
	}
	InstrumentAdmin, InstrumentID, ok := tokenRegistryKey(contract).Parts()
	if !ok {
		return nil, fmt.Errorf("invalid Canton token contract %q, expected <instrument-admin>#<instrument-id>", contract)
	}

	requestedAt := time.Now().UTC().Truncate(time.Microsecond)
	executeBefore := requestedAt.Add(24 * time.Hour)
	inputHoldingCIDs := tokenTransferInputHoldingCIDs(senderHoldings, string(args.GetFrom()), InstrumentAdmin, InstrumentID, requestedAt)
	if len(inputHoldingCIDs) == 0 {
		return nil, fmt.Errorf("no visible token holdings found for sender %s and %s#%s", args.GetFrom(), InstrumentAdmin, InstrumentID)
	}

	choiceArgs := map[string]any{
		"expectedAdmin": InstrumentAdmin,
		"transfer": map[string]any{
			"sender":           string(args.GetFrom()),
			"receiver":         string(args.GetTo()),
			"amount":           transferAmountNumeric(args, decimals),
			"instrumentId":     map[string]any{"admin": InstrumentAdmin, "id": InstrumentID},
			"requestedAt":      requestedAt.Format(time.RFC3339Nano),
			"executeBefore":    executeBefore.Format(time.RFC3339Nano),
			"inputHoldingCids": inputHoldingCIDs,
			"meta":             map[string]any{"values": tokenMetadataValues(args)},
		},
		"extraArgs": map[string]any{
			"context": map[string]any{"values": map[string]any{}},
			"meta":    map[string]any{"values": map[string]string{}},
		},
	}
	tryRegistry := func() (*interactive.PrepareSubmissionResponse, error) {
		registryBaseURL, registryToken, err := client.resolveTokenRegistryBaseURL(ctx, contract, InstrumentAdmin)
		if err != nil {
			return nil, err
		}
		packageMap, err := client.LedgerClient.ListKnownPackageIDsByName(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve package id map for token transfer disclosures: %w", err)
		}
		registryContext, err := client.LedgerClient.GetTokenTransferFactoryAt(ctx, registryToken, registryBaseURL, choiceArgs)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve token transfer factory via registry: %w", err)
		}
		disclosedContracts, registrySynchronizerID, err := tokenDisclosedContractsToProto(registryContext.ChoiceContext.DisclosedContracts, packageMap)
		if err != nil {
			return nil, fmt.Errorf("failed to convert token transfer disclosures: %w", err)
		}
		cmd, err := BuildTokenStandardTransferCommand(
			args,
			transferPackageID,
			registryContext.FactoryID,
			registryContext.ChoiceContext.ChoiceContextData,
			senderHoldings,
			decimals,
			requestedAt,
			executeBefore,
		)
		if err != nil {
			return nil, err
		}
		commandID := cantonproto.NewCommandID()
		synchronizerID, err := client.resolveSynchronizerID(ctx, string(args.GetFrom()), registrySynchronizerID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve token transfer synchronizer: %w", err)
		}
		actAs := []string{string(args.GetFrom())}
		readAs := []string{string(args.GetFrom())}
		prepareReq := cantonproto.NewPrepareRequest(commandID, synchronizerID, actAs, readAs, []*v2.Command{cmd}, disclosedContracts)
		client.logger().WithFields(logrus.Fields{
			"mode":                   "registry",
			"contract":               contract,
			"sender":                 args.GetFrom(),
			"receiver":               args.GetTo(),
			"amount":                 transferAmountNumeric(args, decimals),
			"input_holding_cids":     inputHoldingCIDs,
			"factory_contract_id":    registryContext.FactoryID,
			"transfer_kind":          registryContext.TransferKind,
			"command_id":             commandID,
			"synchronizer_id":        synchronizerID,
			"act_as":                 actAs,
			"read_as":                readAs,
			"disclosed_contract_ids": disclosedContractIDs(disclosedContracts),
		}).Debug("canton token transfer: preparing registry-backed submission")
		prepareResp, err := client.LedgerClient.PrepareSubmission(ctx, prepareReq)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare registry-backed TransferFactory_Transfer: %w", err)
		}
		return prepareResp, nil
	}

	tryLedgerFallback := func() (*interactive.PrepareSubmissionResponse, error) {
		ledgerEnd, err := client.LedgerClient.GetLedgerEnd(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get ledger end for token admin contracts: %w", err)
		}
		transferPackageID, err := client.LedgerClient.ResolvePackageIDByName(ctx, "splice-api-token-transfer-instruction-v1")
		if err != nil {
			return nil, fmt.Errorf("failed to resolve token transfer interface package for ledger fallback: %w", err)
		}
		adminContracts, err := client.LedgerClient.GetTokenTransferFactoryContracts(ctx, InstrumentAdmin, ledgerEnd, transferPackageID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch token transfer factory contracts for %s: %w", InstrumentAdmin, err)
		}
		factoryCreated, err := resolveLedgerTokenTransferFactoryContract(adminContracts, InstrumentAdmin, InstrumentID)
		if err != nil {
			return nil, err
		}
		cmd, err := BuildTokenStandardTransferCommand(
			args,
			transferPackageID,
			factoryCreated.GetContractId(),
			map[string]any{"values": map[string]any{}},
			senderHoldings,
			decimals,
			requestedAt,
			executeBefore,
		)
		if err != nil {
			return nil, err
		}
		commandID := cantonproto.NewCommandID()
		synchronizerID, err := client.resolveSynchronizerID(ctx, string(args.GetFrom()), "")
		if err != nil {
			return nil, fmt.Errorf("failed to resolve token transfer synchronizer: %w", err)
		}
		disclosedContracts := []*v2.DisclosedContract{{
			TemplateId:       factoryCreated.GetTemplateId(),
			ContractId:       factoryCreated.GetContractId(),
			CreatedEventBlob: factoryCreated.GetCreatedEventBlob(),
		}}
		readAs := []string{string(args.GetFrom())}
		if InstrumentAdmin != string(args.GetFrom()) {
			readAs = append(readAs, InstrumentAdmin)
		}
		prepareReq := cantonproto.NewPrepareRequest(commandID, synchronizerID, []string{string(args.GetFrom())}, readAs, []*v2.Command{cmd}, disclosedContracts)
		client.logger().WithFields(logrus.Fields{
			"mode":                   "ledger-fallback",
			"contract":               contract,
			"sender":                 args.GetFrom(),
			"receiver":               args.GetTo(),
			"amount":                 transferAmountNumeric(args, decimals),
			"input_holding_cids":     inputHoldingCIDs,
			"factory_contract_id":    factoryCreated.GetContractId(),
			"factory_template":       fmt.Sprintf("%s:%s", factoryCreated.GetTemplateId().GetModuleName(), factoryCreated.GetTemplateId().GetEntityName()),
			"command_id":             commandID,
			"synchronizer_id":        synchronizerID,
			"act_as":                 []string{string(args.GetFrom())},
			"read_as":                readAs,
			"disclosed_contract_ids": disclosedContractIDs(disclosedContracts),
		}).Debug("canton token transfer: preparing ledger-backed submission")
		prepareResp, err := client.LedgerClient.PrepareSubmission(ctx, prepareReq)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare ledger-backed TransferFactory_Transfer: %w", err)
		}
		return prepareResp, nil
	}

	prepareResp, err := tryRegistry()
	if err == nil {
		return prepareResp, nil
	}
	client.logger().WithError(err).WithField("contract", contract).Warn("registry-backed token transfer preparation failed, falling back to ledger token factory discovery")

	prepareResp, fallbackErr := tryLedgerFallback()
	if fallbackErr != nil {
		return nil, fmt.Errorf("failed token transfer preparation via registry (%v) and ledger fallback (%w)", err, fallbackErr)
	}
	return prepareResp, nil
}

func (client *Client) PrepareNativeTokenStandardTransferCommand(
	ctx context.Context,
	args xcbuilder.TransferArgs,
	uiToken string,
	amuletRules AmuletRules,
	senderHoldings []*v2.ActiveContract,
	decimals int32,
) (*interactive.PrepareSubmissionResponse, xc.ContractAddress, error) {
	transferPackageID, err := client.LedgerClient.ResolvePackageIDByName(ctx, "splice-api-token-transfer-instruction-v1")
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve token transfer interface package: %w", err)
	}
	packageMap, err := client.LedgerClient.ListKnownPackageIDsByName(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve package id map for native token-standard transfer disclosures: %w", err)
	}

	InstrumentAdmin := amuletRules.AmuletRulesUpdate.Contract.Payload.DSO
	if InstrumentAdmin == "" {
		return nil, "", fmt.Errorf("amulet rules missing DSO party")
	}
	const InstrumentID = "Amulet"
	contract := xc.ContractAddress(InstrumentAdmin + "#" + InstrumentID)

	requestedAt := time.Now().UTC().Truncate(time.Microsecond)
	executeBefore := requestedAt.Add(24 * time.Hour)
	inputHoldingCIDs := tokenTransferInputHoldingCIDs(senderHoldings, string(args.GetFrom()), InstrumentAdmin, InstrumentID, requestedAt)
	if len(inputHoldingCIDs) == 0 {
		return nil, "", fmt.Errorf("no visible token-standard Amulet holdings found for sender %s", args.GetFrom())
	}

	choiceArgs := map[string]any{
		"expectedAdmin": InstrumentAdmin,
		"transfer": map[string]any{
			"sender":           string(args.GetFrom()),
			"receiver":         string(args.GetTo()),
			"amount":           transferAmountNumeric(args, decimals),
			"instrumentId":     map[string]any{"admin": InstrumentAdmin, "id": InstrumentID},
			"requestedAt":      requestedAt.Format(time.RFC3339Nano),
			"executeBefore":    executeBefore.Format(time.RFC3339Nano),
			"inputHoldingCids": inputHoldingCIDs,
			"meta":             map[string]any{"values": tokenMetadataValues(args)},
		},
		"extraArgs": map[string]any{
			"context": map[string]any{"values": map[string]any{}},
			"meta":    map[string]any{"values": map[string]string{}},
		},
	}

	registryContext, err := client.LedgerClient.GetTokenTransferFactory(ctx, uiToken, choiceArgs)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve native Amulet transfer factory via Scan: %w", err)
	}
	disclosedContracts, registrySynchronizerID, err := tokenDisclosedContractsToProto(registryContext.ChoiceContext.DisclosedContracts, packageMap)
	if err != nil {
		return nil, "", fmt.Errorf("failed to convert native Amulet transfer disclosures: %w", err)
	}
	cmd, err := BuildTokenStandardTransferCommandForInstrument(
		args,
		transferPackageID,
		registryContext.FactoryID,
		registryContext.ChoiceContext.ChoiceContextData,
		senderHoldings,
		decimals,
		requestedAt,
		executeBefore,
		InstrumentAdmin,
		InstrumentID,
	)
	if err != nil {
		return nil, "", err
	}

	commandID := cantonproto.NewCommandID()
	synchronizerID, err := client.resolveSynchronizerID(ctx, string(args.GetFrom()), registrySynchronizerID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve native Amulet transfer synchronizer: %w", err)
	}
	actAs := []string{string(args.GetFrom())}
	readAs := []string{string(args.GetFrom())}
	prepareReq := cantonproto.NewPrepareRequest(commandID, synchronizerID, actAs, readAs, []*v2.Command{cmd}, disclosedContracts)
	client.logger().WithFields(logrus.Fields{
		"mode":                   "native-token-standard",
		"contract":               contract,
		"sender":                 args.GetFrom(),
		"receiver":               args.GetTo(),
		"amount":                 transferAmountNumeric(args, decimals),
		"input_holding_cids":     inputHoldingCIDs,
		"factory_contract_id":    registryContext.FactoryID,
		"transfer_kind":          registryContext.TransferKind,
		"command_id":             commandID,
		"synchronizer_id":        synchronizerID,
		"act_as":                 actAs,
		"read_as":                readAs,
		"disclosed_contract_ids": disclosedContractIDs(disclosedContracts),
	}).Debug("canton native Amulet transfer: preparing token-standard submission")
	prepareResp, err := client.LedgerClient.PrepareSubmission(ctx, prepareReq)
	if err != nil {
		return nil, "", fmt.Errorf("failed to prepare native Amulet TransferFactory_Transfer: %w", err)
	}
	return prepareResp, contract, nil
}

func (client *Client) PrepareNativeTokenStandardTransferFromLedger(
	ctx context.Context,
	args xcbuilder.TransferArgs,
	uiToken string,
	amuletRules AmuletRules,
	ledgerEnd int64,
) (*interactive.PrepareSubmissionResponse, xc.ContractAddress, error) {
	holdingPackageID, err := client.LedgerClient.ResolvePackageIDByName(ctx, "splice-api-token-holding-v1")
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve token holding interface package: %w", err)
	}
	senderHoldings, err := client.LedgerClient.GetTokenHoldingContracts(ctx, string(args.GetFrom()), ledgerEnd, holdingPackageID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch token-standard Amulet holdings: %w", err)
	}
	return client.PrepareNativeTokenStandardTransferCommand(ctx, args, uiToken, amuletRules, senderHoldings, client.Asset.GetChain().Decimals)
}

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	input := tx_input.NewTxInput()
	from := args.GetFrom()
	// to := args.GetTo()

	ledgerEnd, err := client.LedgerClient.GetLedgerEnd(ctx)
	if err != nil {
		return input, fmt.Errorf("failed to get ledger end: %w", err)
	}
	input.LedgerEnd = ledgerEnd

	senderContracts, err := client.LedgerClient.GetActiveContracts(ctx, string(from), ledgerEnd, true)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active contracts: %w", err)
	}

	if contract, ok := args.GetContract(); ok && !client.Asset.GetChain().IsChain(contract) {
		holdingPackageID, err := client.LedgerClient.ResolvePackageIDByName(ctx, "splice-api-token-holding-v1")
		if err != nil {
			return nil, fmt.Errorf("failed to resolve token holding interface package: %w", err)
		}
		senderHoldings, err := client.LedgerClient.GetTokenHoldingContracts(ctx, string(from), ledgerEnd, holdingPackageID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch token holding contracts: %w", err)
		}
		decimals, err := resolveTransferTokenDecimals(ctx, client, args, contract)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve token decimals: %w", err)
		}
		resp, err := client.PrepareTokenTransferCommand(ctx, args, senderContracts, senderHoldings, int32(decimals))
		if err != nil {
			return nil, fmt.Errorf("failed to prepare token transfer command: %w", err)
		}

		input.PreparedTransaction = resp.GetPreparedTransaction()
		input.SubmissionId = NewCommandId()
		input.HashingSchemeVersion = resp.GetHashingSchemeVersion()
		input.DeduplicationWindow = cantonproto.ResolveDeduplicationWindow(client.Asset.TransactionActiveTime)
		input.Decimals = int32(decimals)
		return input, nil
	}

	uiToken, err := client.cantonUIToken(ctx)
	if err != nil {
		return nil, err
	}

	amuletRules, err := client.LedgerClient.GetAmuletRules(ctx, uiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch amulet rules: %w", err)
	}

	txResp, contract, err := client.PrepareNativeTokenStandardTransferFromLedger(ctx, args, uiToken, *amuletRules, ledgerEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare token standard transfer: %w", err)
	}
	input.PreparedTransaction = txResp.GetPreparedTransaction()
	input.SubmissionId = NewCommandId()
	input.HashingSchemeVersion = txResp.GetHashingSchemeVersion()
	input.DeduplicationWindow = cantonproto.ResolveDeduplicationWindow(client.Asset.TransactionActiveTime)
	input.Decimals = client.Asset.GetChain().Decimals
	input.ContractAddress = contract
	return input, nil
}

// FetchLegacyTxInput - Deprecated, use FetchTransferInput
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	chainCfg := client.Asset.GetChain().Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx accepts a serialized Canton transaction together with metadata that
// identifies the Canton transaction type to submit.
func (client *Client) SubmitTx(ctx context.Context, submitReq xctypes.SubmitTxReq) error {
	if len(submitReq.TxData) == 0 {
		return fmt.Errorf("empty transaction data")
	}
	if submitReq.BroadcastInput == "" {
		return fmt.Errorf("missing Canton tx metadata")
	}
	metadata, err := cantontx.ParseMetadata([]byte(submitReq.BroadcastInput))
	if err != nil {
		return fmt.Errorf("failed to parse Canton tx metadata: %w", err)
	}
	switch metadata.TxType {
	case cantontx.TxTypeCreateAccount:
		createAccountTx, err := cantontx.ParseCreateAccountTxWithMetadata(submitReq.TxData, metadata)
		if err != nil {
			return fmt.Errorf("failed to parse Canton create-account tx: %w", err)
		}
		return client.submitCreateAccountTx(ctx, createAccountTx)
	case cantontx.TxTypeTransfer:
		return client.submitTransferTx(ctx, submitReq.TxData)
	default:
		return fmt.Errorf("unsupported Canton tx type %q", metadata.TxType)
	}
}

func (client *Client) submitTransferTx(ctx context.Context, payload []byte) error {
	var req interactive.ExecuteSubmissionAndWaitRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return fmt.Errorf("failed to unmarshal Canton execute-and-wait request: %w", err)
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
		"hashing":       req.HashingSchemeVersion.String(),
	}).Trace("canton request")

	_, err := client.LedgerClient.ExecuteSubmissionAndWait(ctx, &req)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"submission_id": req.SubmissionId,
			"parties":       parties,
			"hashing":       req.HashingSchemeVersion.String(),
		}).WithError(err).Warn("canton token transfer submit failed")
		return fmt.Errorf("failed to submit Canton transaction: %w", err)
	}
	logrus.WithField("submission_id", req.SubmissionId).Trace("canton response: ExecuteSubmissionAndWait accepted")
	return err
}

// FetchLegacyTxInfo - not implemented for Canton
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	return txinfo.LegacyTxInfo{}, errors.New("not implemented")
}

type amuletCreation struct {
	Owner  string
	Amount xc.AmountBlockchain
}

type TokenHoldingCreation struct {
	NodeID          int32
	Owner           string
	InstrumentAdmin string
	InstrumentID    string
	Amount          string
}

type tokenMovement struct {
	from     string
	to       string
	contract xc.ContractAddress
	Amount   xc.AmountBlockchain
	Memo     string
}

// FetchTxInfo fetches and normalizes transaction info for a Canton update by its updateId.
// Recovery tokens in the form "<ledger_end>-<submission_id>" are resolved via the completion stream.
func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	lookupId := string(args.TxHash())
	sender, hasSender := args.Sender()

	updateId := lookupId
	if beginExclusive, submissionId, ok := parseRecoveryLookupId(lookupId); ok {
		if !hasSender {
			return txinfo.TxInfo{}, fmt.Errorf("canton recovery lookup for %q requires sender address", lookupId)
		}
		resolvedUpdateId, err := client.LedgerClient.RecoverUpdateIdBySubmissionId(ctx, beginExclusive, string(sender), submissionId)
		if err != nil {
			return txinfo.TxInfo{}, fmt.Errorf("failed to resolve Canton recovery token %q: %w", lookupId, err)
		}
		updateId = resolvedUpdateId
	}
	if hasSender {
		return client.fetchTxInfoFromLedger(ctx, updateId, lookupId, string(sender))
	}

	candidates, err := client.LedgerClient.GetLighthouseSubmittingPartiesByID(ctx, updateId)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("canton tx-info lookup for %q requires sender address and Lighthouse sender lookup failed: %w", lookupId, err)
	}
	return client.fetchTxInfoFromLedgerCandidates(ctx, updateId, lookupId, candidates)
}

func (client *Client) fetchTxInfoFromLedgerCandidates(ctx context.Context, updateId string, lookupId string, candidates []string) (txinfo.TxInfo, error) {
	if len(candidates) == 0 {
		return txinfo.TxInfo{}, fmt.Errorf("Lighthouse did not return a submitting party for update %s", updateId)
	}
	var failures []string
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		info, err := client.fetchTxInfoFromLedger(ctx, updateId, lookupId, candidate)
		if err == nil {
			return info, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", candidate, err))
	}
	if len(failures) == 0 {
		return txinfo.TxInfo{}, fmt.Errorf("Lighthouse did not return a non-empty submitting party for update %s", updateId)
	}
	return txinfo.TxInfo{}, fmt.Errorf("failed to fetch update with Lighthouse-resolved parties: %s", strings.Join(failures, "; "))
}

func (client *Client) fetchTxInfoFromLedger(ctx context.Context, updateId string, lookupId string, queryingParty string) (txinfo.TxInfo, error) {
	chainCfg := client.Asset.GetChain()
	decimals := chainCfg.Decimals
	resp, err := client.LedgerClient.GetUpdateById(ctx, queryingParty, updateId)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to fetch update: %w", err)
	}

	tx := resp.GetTransaction()
	if tx == nil {
		return txinfo.TxInfo{}, fmt.Errorf("update %s is not a transaction", updateId)
	}

	ledgerEnd, err := client.LedgerClient.GetLedgerEnd(ctx)
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
	blockIdentifier := fmt.Sprintf("%s/%d", tx.GetSynchronizerId(), txOffset)
	block := txinfo.NewBlock(chainCfg.Chain, uint64(txOffset), blockIdentifier, blockTime)
	hash := updateId
	if _, _, ok := parseRecoveryLookupId(lookupId); ok {
		hash = lookupId
	}
	txInfo := txinfo.NewTxInfo(block, client.Asset, hash, confirmations, nil)
	txInfo.LookupId = updateId

	// Use the ledger events as the source of truth for Canton tx-info movement reconstruction.
	var senderParty string
	zero := xc.NewAmountBlockchainFromUint64(0)
	var transferOutputs []amuletCreation
	totalFee := xc.NewAmountBlockchainFromUint64(0)
	var amuletCreations []amuletCreation
	var tokenHoldingCreations []TokenHoldingCreation
	var tokenMovements []tokenMovement
	tokenDecimalsCache := make(map[xc.ContractAddress]int32)
	resolveTokenDecimals := func(contract xc.ContractAddress) int32 {
		if decimals, ok := tokenDecimalsCache[contract]; ok {
			return decimals
		}
		if assetCfg, ok := client.Asset.FindAdditionalNativeAsset(contract); ok && assetCfg.Decimals > 0 {
			tokenDecimalsCache[contract] = int32(assetCfg.Decimals)
			return int32(assetCfg.Decimals)
		}
		decimals, err := client.FetchDecimals(ctx, contract)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"contract":  contract,
				"update_id": updateId,
			}).WithError(err).Debug("falling back to chain decimals for Canton token tx-info")
			tokenDecimalsCache[contract] = chainCfg.Decimals
			return chainCfg.Decimals
		}
		tokenDecimalsCache[contract] = int32(decimals)
		return int32(decimals)
	}

	for _, event := range tx.GetEvents() {
		if cr := event.GetCreated(); cr != nil {
			Owner, InstrumentAdmin, InstrumentID, Amount, ok := ExtractTokenHoldingView(cr)
			if ok {
				tokenHoldingCreations = append(tokenHoldingCreations, TokenHoldingCreation{
					NodeID:          cr.GetNodeId(),
					Owner:           Owner,
					InstrumentAdmin: InstrumentAdmin,
					InstrumentID:    InstrumentID,
					Amount:          Amount,
				})
			}
		}
	}

	for _, event := range tx.GetEvents() {
		if ex := event.GetExercised(); ex != nil {
			movements, err := client.ExtractTokenMovementsFromExercise(
				ctx,
				queryingParty,
				ex,
				tokenHoldingCreations,
				resolveTokenDecimals,
			)
			if err != nil {
				logrus.WithField("update_id", updateId).WithError(err).Debug("failed to reconstruct Canton token movements from tx-info events")
			} else {
				tokenMovements = append(tokenMovements, movements...)
			}
			if eventSender, ok := ExtractTransferSender(ex); ok && senderParty == "" {
				senderParty = eventSender
			}
			if outputs, ok := extractTransferOutputs(ex, decimals); ok {
				transferOutputs = append(transferOutputs, outputs...)
			}
			if fee, ok := ExtractTransferFee(ex, decimals); ok {
				totalFee = totalFee.Add(&fee)
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
			// Extract Owner and initialAmount from Amulet contract
			var Owner string
			for _, f := range createArgs.GetFields() {
				if f.GetLabel() == "owner" {
					Owner = f.GetValue().GetParty()
				}
			}
			if Owner == "" {
				continue
			}
			bal, ok := ExtractAmuletBalance(createArgs, decimals)
			if !ok {
				continue
			}
			amuletCreations = append(amuletCreations, amuletCreation{Owner: Owner, Amount: bal})
		}
	}

	for _, movement := range tokenMovements {
		txInfo.AddSimpleTransfer(
			xc.Address(movement.from),
			xc.Address(movement.to),
			movement.contract,
			movement.Amount,
			nil,
			movement.Memo,
		)
	}

	if len(transferOutputs) > 0 {
		if senderParty == "" {
			return txinfo.TxInfo{}, fmt.Errorf("could not determine Canton transfer sender from events for update %s", updateId)
		}
		for _, out := range transferOutputs {
			txInfo.AddSimpleTransfer(xc.Address(senderParty), xc.Address(out.Owner), "", out.Amount, nil, "")
		}
		if totalFee.Cmp(&zero) > 0 {
			txInfo.AddFee(xc.Address(senderParty), "", totalFee, nil)
		}
		txInfo.Fees = txInfo.CalculateFees()
		txInfo.SyncDeprecatedFields()
		return *txInfo, nil
	}

	// Fall back to created Amulets only when there is no explicit transfer exercise payload.
	if senderParty == "" {
		txInfo.Fees = txInfo.CalculateFees()
		txInfo.SyncDeprecatedFields()
		return *txInfo, nil
	}
	for _, ac := range amuletCreations {
		if ac.Owner == senderParty {
			continue
		}
		from := xc.Address(senderParty)
		to := xc.Address(ac.Owner)
		txInfo.AddSimpleTransfer(from, to, "", ac.Amount, nil, "")
	}

	// If we couldn't distinguish sender from receiver (e.g. self-transfer), fall back to
	// reporting all sender-visible created Amulets.
	if len(txInfo.Movements) == 0 && len(amuletCreations) > 0 {
		for _, ac := range amuletCreations {
			from := xc.Address(senderParty)
			to := xc.Address(ac.Owner)
			txInfo.AddSimpleTransfer(from, to, "", ac.Amount, nil, "")
		}
	}
	if totalFee.Cmp(&zero) > 0 {
		txInfo.AddFee(xc.Address(senderParty), "", totalFee, nil)
	}

	txInfo.Fees = txInfo.CalculateFees()
	txInfo.SyncDeprecatedFields()
	return *txInfo, nil
}

func (client *Client) ExtractTokenMovementsFromExercise(
	ctx context.Context,
	queryingParty string,
	ex *v2.ExercisedEvent,
	createdHoldings []TokenHoldingCreation,
	resolveTokenDecimals func(contract xc.ContractAddress) int32,
) ([]tokenMovement, error) {
	if ex == nil {
		return nil, nil
	}

	var sender string
	var memo string
	switch {
	case isTokenTransferFactoryExercise(ex):
		transfer, ok := extractTokenTransferRecordFromValue(ex.GetChoiceArgument())
		if !ok {
			return nil, nil
		}
		return tokenMovementFromTransferView(transfer, resolveTokenDecimals), nil
	case isTokenTransferInstructionExercise(ex):
		if queryingParty == "" {
			return nil, nil
		}
		resp, err := client.LedgerClient.GetEventsByContractID(ctx, queryingParty, ex.GetContractId())
		if err != nil {
			return nil, err
		}
		created := resp.GetCreated()
		if created == nil || created.GetCreatedEvent() == nil {
			return nil, nil
		}
		transfer, ok := extractTokenTransferInstructionView(created.GetCreatedEvent())
		if !ok {
			return nil, nil
		}
		sender = transfer.sender
		memo = transfer.Memo
	default:
		return nil, nil
	}

	if sender == "" {
		return nil, nil
	}

	movements := make([]tokenMovement, 0)
	for _, create := range createdHoldings {
		if create.NodeID <= ex.GetNodeId() || create.NodeID > ex.GetLastDescendantNodeId() {
			continue
		}
		if create.Owner == "" || create.Owner == sender {
			continue
		}
		contract := xc.ContractAddress(create.InstrumentAdmin + "#" + create.InstrumentID)
		Amount, ok := parseHumanAmountToBlockchain(create.Amount, resolveTokenDecimals(contract))
		if !ok {
			continue
		}
		movements = append(movements, tokenMovement{
			from:     sender,
			to:       create.Owner,
			contract: contract,
			Amount:   Amount,
			Memo:     memo,
		})
	}
	return movements, nil
}

func tokenMovementFromTransferView(
	transfer tokenTransferView,
	resolveTokenDecimals func(contract xc.ContractAddress) int32,
) []tokenMovement {
	if transfer.sender == "" || transfer.receiver == "" || transfer.receiver == transfer.sender {
		return nil
	}
	if transfer.InstrumentAdmin == "" || transfer.InstrumentID == "" || transfer.Amount == "" {
		return nil
	}
	contract := xc.ContractAddress(transfer.InstrumentAdmin + "#" + transfer.InstrumentID)
	movementContract := contract
	decimalsContract := contract
	if transfer.InstrumentID == "Amulet" {
		movementContract = ""
		decimalsContract = xc.ContractAddress(xc.CANTON)
	}
	amount, ok := parseHumanAmountToBlockchain(transfer.Amount, resolveTokenDecimals(decimalsContract))
	if !ok {
		return nil
	}
	return []tokenMovement{{
		from:     transfer.sender,
		to:       transfer.receiver,
		contract: movementContract,
		Amount:   amount,
		Memo:     transfer.Memo,
	}}
}

func ExtractTransferSender(ex *v2.ExercisedEvent) (string, bool) {
	if !isTransferExercise(ex) {
		return "", false
	}
	if len(ex.GetActingParties()) > 0 && ex.GetActingParties()[0] != "" {
		return ex.GetActingParties()[0], true
	}

	arg := ex.GetChoiceArgument()
	if arg == nil {
		return "", false
	}
	record := arg.GetRecord()
	if record == nil {
		return "", false
	}
	if senderValue, ok := findValueField(record, "sender"); ok {
		if sender := senderValue.GetParty(); sender != "" {
			return sender, true
		}
	}
	return "", false
}

func extractTransferOutputs(ex *v2.ExercisedEvent, decimals int32) ([]amuletCreation, bool) {
	if !isTransferExercise(ex) {
		return nil, false
	}

	arg := ex.GetChoiceArgument()
	if arg == nil {
		return nil, false
	}
	root := arg.GetRecord()
	if root == nil {
		return nil, false
	}

	transferRecord := findRecordField(root, "transfer")
	if transferRecord == nil {
		return nil, false
	}

	var outputs []*v2.Value
	for _, field := range transferRecord.GetFields() {
		if field.GetLabel() == "outputs" {
			if list := field.GetValue().GetList(); list != nil {
				outputs = list.GetElements()
			}
			break
		}
	}
	if len(outputs) == 0 {
		return nil, false
	}

	parsed := make([]amuletCreation, 0, len(outputs))
	for _, output := range outputs {
		record := output.GetRecord()
		if record == nil {
			continue
		}
		var receiver string
		var Amount xc.AmountBlockchain
		var ok bool
		for _, field := range record.GetFields() {
			switch field.GetLabel() {
			case "receiver":
				receiver = field.GetValue().GetParty()
			case "amount":
				Amount, ok = extractNumericValue(field.GetValue(), decimals)
			}
		}
		if receiver == "" || !ok {
			continue
		}
		parsed = append(parsed, amuletCreation{Owner: receiver, Amount: Amount})
	}
	if len(parsed) == 0 {
		return nil, false
	}
	return parsed, true
}

func extractNumericValue(value *v2.Value, decimals int32) (xc.AmountBlockchain, bool) {
	if value == nil {
		return xc.AmountBlockchain{}, false
	}
	numeric := value.GetNumeric()
	if numeric == "" {
		return xc.AmountBlockchain{}, false
	}
	human, err := xc.NewAmountHumanReadableFromStr(numeric)
	if err != nil {
		return xc.AmountBlockchain{}, false
	}
	return human.ToBlockchain(decimals), true
}

func ExtractTransferFee(ex *v2.ExercisedEvent, decimals int32) (xc.AmountBlockchain, bool) {
	if !isTransferExercise(ex) {
		return xc.AmountBlockchain{}, false
	}

	result := ex.GetExerciseResult()
	if result == nil {
		return xc.AmountBlockchain{}, false
	}
	record := result.GetRecord()
	if record == nil {
		return xc.AmountBlockchain{}, false
	}
	record = unwrapTransferResultRecord(record)

	if burned, ok := extractBurnedFee(record, decimals); ok {
		return burned, true
	}
	if summaryFee, ok := extractSummaryFee(record, decimals); ok {
		return summaryFee, true
	}
	return xc.AmountBlockchain{}, false
}

func isTokenTransferFactoryExercise(ex *v2.ExercisedEvent) bool {
	if ex == nil || ex.GetChoice() != "TransferFactory_Transfer" {
		return false
	}
	interfaceID := ex.GetInterfaceId()
	if interfaceID != nil {
		return interfaceID.GetModuleName() == TokenTransferModule && interfaceID.GetEntityName() == TokenTransferEntity
	}
	templateID := ex.GetTemplateId()
	return templateID != nil && templateID.GetModuleName() == TokenTransferModule && templateID.GetEntityName() == TokenTransferEntity
}

func isTokenTransferInstructionExercise(ex *v2.ExercisedEvent) bool {
	if ex == nil {
		return false
	}
	switch ex.GetChoice() {
	case "TransferInstruction_Accept", "TransferInstruction_Reject", "TransferInstruction_Withdraw":
	default:
		return false
	}
	interfaceID := ex.GetInterfaceId()
	if interfaceID != nil {
		return interfaceID.GetModuleName() == TokenTransferModule && interfaceID.GetEntityName() == "TransferInstruction"
	}
	templateID := ex.GetTemplateId()
	return templateID != nil && templateID.GetModuleName() == TokenTransferModule && templateID.GetEntityName() == "TransferInstruction"
}

type tokenTransferView struct {
	sender          string
	receiver        string
	InstrumentAdmin string
	InstrumentID    string
	Amount          string
	Memo            string
}

func extractTokenTransferInstructionView(created *v2.CreatedEvent) (tokenTransferView, bool) {
	if created == nil {
		return tokenTransferView{}, false
	}
	for _, view := range created.GetInterfaceViews() {
		interfaceID := view.GetInterfaceId()
		if interfaceID == nil || interfaceID.GetModuleName() != TokenTransferModule || interfaceID.GetEntityName() != "TransferInstruction" {
			continue
		}
		return extractTokenTransferRecord(view.GetViewValue())
	}
	if created.GetCreateArguments() != nil {
		return extractTokenTransferRecord(created.GetCreateArguments())
	}
	return tokenTransferView{}, false
}

func extractTokenTransferRecordFromValue(value *v2.Value) (tokenTransferView, bool) {
	if value == nil || value.GetRecord() == nil {
		return tokenTransferView{}, false
	}
	return extractTokenTransferRecord(value.GetRecord())
}

func extractTokenTransferRecord(record *v2.Record) (tokenTransferView, bool) {
	transferRecord := findRecordField(record, "transfer")
	if transferRecord == nil {
		return tokenTransferView{}, false
	}

	senderValue, hasSender := findValueField(transferRecord, "sender")
	receiverValue, hasReceiver := findValueField(transferRecord, "receiver")
	amountValue, hasAmount := findValueField(transferRecord, "amount")
	instrumentValue, hasInstrument := findValueField(transferRecord, "instrumentId")
	if !hasSender || !hasReceiver || !hasAmount || !hasInstrument || instrumentValue.GetRecord() == nil {
		return tokenTransferView{}, false
	}

	adminValue, hasAdmin := findValueField(instrumentValue.GetRecord(), "admin")
	idValue, hasID := findValueField(instrumentValue.GetRecord(), "id")
	if !hasAdmin || !hasID {
		return tokenTransferView{}, false
	}

	memo := ""
	if metaRecord := findRecordField(transferRecord, "meta"); metaRecord != nil {
		if value, ok := extractMetadataTextValue(metaRecord, tokenMemoMetadataKey); ok {
			memo = value
		}
	}

	return tokenTransferView{
		sender:          senderValue.GetParty(),
		receiver:        receiverValue.GetParty(),
		InstrumentAdmin: adminValue.GetParty(),
		InstrumentID:    idValue.GetText(),
		Amount:          amountValue.GetNumeric(),
		Memo:            memo,
	}, true
}

func isTransferExercise(ex *v2.ExercisedEvent) bool {
	tid := ex.GetTemplateId()
	if tid == nil || tid.GetModuleName() != "Splice.AmuletRules" {
		return false
	}
	switch ex.GetChoice() {
	case "AmuletRules_Transfer", "TransferPreapproval_Send":
		return true
	default:
		return false
	}
}

func unwrapTransferResultRecord(record *v2.Record) *v2.Record {
	if nested := findRecordField(record, "result"); nested != nil {
		return nested
	}
	return record
}

func extractBurnedFee(record *v2.Record, decimals int32) (xc.AmountBlockchain, bool) {
	metaRecord := findRecordField(record, "meta")
	if metaRecord == nil {
		return xc.AmountBlockchain{}, false
	}
	valuesRecord := findRecordField(metaRecord, "values")
	if valuesRecord == nil {
		return xc.AmountBlockchain{}, false
	}
	burnedText, ok := extractTextMapValue(valuesRecord, "splice.lfdecentralizedtrust.org/burned")
	if !ok || burnedText == "" {
		return xc.AmountBlockchain{}, false
	}
	return parseHumanAmountToBlockchain(burnedText, decimals)
}

func extractSummaryFee(record *v2.Record, decimals int32) (xc.AmountBlockchain, bool) {
	summaryRecord := findRecordField(record, "summary")
	if summaryRecord == nil {
		return xc.AmountBlockchain{}, false
	}

	total := xc.NewAmountBlockchainFromUint64(0)
	found := false

	if senderChangeFeeValue, ok := findValueField(summaryRecord, "senderChangeFee"); ok {
		if fee, ok := extractNumericValue(senderChangeFeeValue, decimals); ok {
			total = total.Add(&fee)
			found = true
		}
	}
	if outputFeesValue, ok := findValueField(summaryRecord, "outputFees"); ok {
		if list := outputFeesValue.GetList(); list != nil {
			for _, elem := range list.GetElements() {
				fee, ok := extractNumericValue(elem, decimals)
				if !ok {
					continue
				}
				total = total.Add(&fee)
				found = true
			}
		}
	}

	if !found {
		return xc.AmountBlockchain{}, false
	}
	return total, true
}

func findRecordField(record *v2.Record, label string) *v2.Record {
	value, ok := findValueField(record, label)
	if !ok {
		return nil
	}
	return value.GetRecord()
}

func findValueField(record *v2.Record, label string) (*v2.Value, bool) {
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

func extractTextMapValue(record *v2.Record, key string) (string, bool) {
	for _, field := range record.GetFields() {
		if field.GetLabel() != key {
			continue
		}
		return field.GetValue().GetText(), true
	}
	return "", false
}

func extractMetadataTextValue(metaRecord *v2.Record, key string) (string, bool) {
	valuesValue, ok := findValueField(metaRecord, "values")
	if !ok {
		return "", false
	}
	if textMap := valuesValue.GetTextMap(); textMap != nil {
		for _, entry := range textMap.GetEntries() {
			if entry.GetKey() == key {
				return entry.GetValue().GetText(), true
			}
		}
		return "", false
	}
	if valuesRecord := valuesValue.GetRecord(); valuesRecord != nil {
		return extractTextMapValue(valuesRecord, key)
	}
	return "", false
}

func parseHumanAmountToBlockchain(value string, decimals int32) (xc.AmountBlockchain, bool) {
	human, err := xc.NewAmountHumanReadableFromStr(value)
	if err != nil {
		return xc.AmountBlockchain{}, false
	}
	return human.ToBlockchain(decimals), true
}

func tokenRegistryKey(contract xc.ContractAddress) cantonclientconfig.TokenRegistryKey {
	return cantonclientconfig.TokenRegistryKey(contract)
}

func resolveTransferTokenDecimals(ctx context.Context, client *Client, args xcbuilder.TransferArgs, contract xc.ContractAddress) (int, error) {
	if decimals, ok := args.GetDecimals(); ok {
		return decimals, nil
	}
	return client.FetchDecimals(ctx, contract)
}

func GetRecordFieldValue(record *v2.Record, key string) (*v2.Value, bool) {
	if record == nil {
		return nil, false
	}
	for _, field := range record.GetFields() {
		if field.GetLabel() == key {
			return field.GetValue(), true
		}
	}
	return nil, false
}

func ExtractTokenHoldingView(created *v2.CreatedEvent) (Owner string, InstrumentAdmin string, InstrumentID string, Amount string, ok bool) {
	if created == nil {
		return "", "", "", "", false
	}
	for _, view := range created.GetInterfaceViews() {
		interfaceID := view.GetInterfaceId()
		if interfaceID == nil || interfaceID.GetModuleName() != "Splice.Api.Token.HoldingV1" || interfaceID.GetEntityName() != "Holding" {
			continue
		}
		record := view.GetViewValue()
		ownerValue, ok := GetRecordFieldValue(record, "owner")
		if !ok || ownerValue.GetParty() == "" {
			return "", "", "", "", false
		}
		instrumentValue, ok := GetRecordFieldValue(record, "instrumentId")
		if !ok || instrumentValue.GetRecord() == nil {
			return "", "", "", "", false
		}
		adminValue, ok := GetRecordFieldValue(instrumentValue.GetRecord(), "admin")
		if !ok || adminValue.GetParty() == "" {
			return "", "", "", "", false
		}
		idValue, ok := GetRecordFieldValue(instrumentValue.GetRecord(), "id")
		if !ok || idValue.GetText() == "" {
			return "", "", "", "", false
		}
		amountValue, ok := GetRecordFieldValue(record, "amount")
		if !ok || amountValue.GetNumeric() == "" {
			return "", "", "", "", false
		}
		return ownerValue.GetParty(), adminValue.GetParty(), idValue.GetText(), amountValue.GetNumeric(), true
	}
	if args := created.GetCreateArguments(); args != nil {
		ownerValue, hasOwner := GetRecordFieldValue(args, "owner")
		adminValue, hasAdmin := GetRecordFieldValue(args, "admin")
		amountValue, hasAmount := GetRecordFieldValue(args, "amount")
		if !hasOwner || !hasAdmin || !hasAmount || ownerValue.GetParty() == "" || amountValue.GetNumeric() == "" {
			return "", "", "", "", false
		}
		if symbolValue, hasSymbol := GetRecordFieldValue(args, "symbol"); hasSymbol && symbolValue.GetText() != "" {
			return ownerValue.GetParty(), adminValue.GetParty(), symbolValue.GetText(), amountValue.GetNumeric(), true
		}
		if instrumentValue, hasInstrument := GetRecordFieldValue(args, "instrumentId"); hasInstrument && instrumentValue.GetRecord() != nil {
			instrumentAdminValue, hasInstrumentAdmin := GetRecordFieldValue(instrumentValue.GetRecord(), "admin")
			instrumentIDValue, hasInstrumentID := GetRecordFieldValue(instrumentValue.GetRecord(), "id")
			if hasInstrumentAdmin && hasInstrumentID && instrumentAdminValue.GetParty() != "" && instrumentIDValue.GetText() != "" {
				return ownerValue.GetParty(), instrumentAdminValue.GetParty(), instrumentIDValue.GetText(), amountValue.GetNumeric(), true
			}
		}
	}
	return "", "", "", "", false
}

func extractOfferAmountAndAsset(value *v2.Value) (xc.ContractAddress, string, bool) {
	if value == nil {
		return "", "", false
	}
	if numeric := value.GetNumeric(); numeric != "" {
		return xc.ContractAddress(xc.CANTON), numeric, true
	}
	record := value.GetRecord()
	if record == nil {
		return "", "", false
	}
	if instrumentValue, ok := GetRecordFieldValue(record, "instrumentId"); ok && instrumentValue.GetRecord() != nil {
		adminValue, hasAdmin := GetRecordFieldValue(instrumentValue.GetRecord(), "admin")
		idValue, hasID := GetRecordFieldValue(instrumentValue.GetRecord(), "id")
		amountValue, hasAmount := GetRecordFieldValue(record, "amount")
		if hasAdmin && hasID && hasAmount && adminValue.GetParty() != "" && idValue.GetText() != "" && amountValue.GetNumeric() != "" {
			return xc.ContractAddress(adminValue.GetParty() + "#" + idValue.GetText()), amountValue.GetNumeric(), true
		}
	}
	amountValue, hasAmount := GetRecordFieldValue(record, "amount")
	if !hasAmount {
		return "", "", false
	}
	if amountValue.GetNumeric() != "" {
		if instrumentValue, ok := GetRecordFieldValue(record, "instrument"); ok && instrumentValue.GetRecord() != nil {
			adminValue, hasAdmin := GetRecordFieldValue(instrumentValue.GetRecord(), "source")
			idValue, hasID := GetRecordFieldValue(instrumentValue.GetRecord(), "id")
			if hasAdmin && hasID && adminValue.GetParty() != "" && idValue.GetText() != "" {
				return xc.ContractAddress(adminValue.GetParty() + "#" + idValue.GetText()), amountValue.GetNumeric(), true
			}
		}
		return xc.ContractAddress(xc.CANTON), amountValue.GetNumeric(), true
	}
	if amountRecord := amountValue.GetRecord(); amountRecord != nil {
		numericValue, hasNumeric := GetRecordFieldValue(amountRecord, "amount")
		if !hasNumeric || numericValue.GetNumeric() == "" {
			return "", "", false
		}
		if unitValue, ok := GetRecordFieldValue(amountRecord, "unit"); ok {
			if enumValue := unitValue.GetEnum(); enumValue != nil && enumValue.GetConstructor() == "AmuletUnit" {
				return xc.ContractAddress(xc.CANTON), numericValue.GetNumeric(), true
			}
			if unitRecord := unitValue.GetRecord(); unitRecord != nil {
				adminValue, hasAdmin := GetRecordFieldValue(unitRecord, "admin")
				idValue, hasID := GetRecordFieldValue(unitRecord, "id")
				if hasAdmin && hasID && adminValue.GetParty() != "" && idValue.GetText() != "" {
					return xc.ContractAddress(adminValue.GetParty() + "#" + idValue.GetText()), numericValue.GetNumeric(), true
				}
			}
		}
		return xc.ContractAddress(xc.CANTON), numericValue.GetNumeric(), true
	}
	return "", "", false
}

func extractLedgerOffer(created *v2.CreatedEvent) (from xc.Address, to xc.Address, assetID xc.ContractAddress, Amount string, expiresAt *time.Time, trackingID string, ok bool) {
	if created == nil {
		return "", "", "", "", nil, "", false
	}
	args := created.GetCreateArguments()
	if args == nil {
		return "", "", "", "", nil, "", false
	}

	var senderValue, receiverValue, amountValue, transferValue *v2.Value
	if value, found := GetRecordFieldValue(args, "sender"); found {
		senderValue = value
	}
	if value, found := GetRecordFieldValue(args, "receiver"); found {
		receiverValue = value
	}
	if value, found := GetRecordFieldValue(args, "amount"); found {
		amountValue = value
	}
	if senderValue == nil || receiverValue == nil || amountValue == nil {
		if value, found := GetRecordFieldValue(args, "transfer"); found && value.GetRecord() != nil {
			transferValue = value
			transferRecord := transferValue.GetRecord()
			senderValue, _ = GetRecordFieldValue(transferRecord, "sender")
			receiverValue, _ = GetRecordFieldValue(transferRecord, "receiver")
			amountValue = transferValue
		}
	}
	if senderValue == nil || receiverValue == nil || amountValue == nil || senderValue.GetParty() == "" || receiverValue.GetParty() == "" {
		return "", "", "", "", nil, "", false
	}

	assetID, Amount, ok = extractOfferAmountAndAsset(amountValue)
	if !ok {
		return "", "", "", "", nil, "", false
	}

	if trackingValue, found := GetRecordFieldValue(args, "trackingId"); found {
		trackingID = trackingValue.GetText()
	}
	if expiresValue, found := GetRecordFieldValue(args, "expiresAt"); found {
		if ts := expiresValue.GetTimestamp(); ts != 0 {
			expiry := time.UnixMicro(ts).UTC()
			expiresAt = &expiry
		}
	}
	if expiresAt == nil && transferValue != nil {
		if executeBeforeValue, found := GetRecordFieldValue(transferValue.GetRecord(), "executeBefore"); found {
			if ts := executeBeforeValue.GetTimestamp(); ts != 0 {
				expiry := time.UnixMicro(ts).UTC()
				expiresAt = &expiry
			}
		}
	}

	return xc.Address(senderValue.GetParty()), xc.Address(receiverValue.GetParty()), assetID, Amount, expiresAt, trackingID, true
}

func (client *Client) resolveOfferAmount(ctx context.Context, assetID xc.ContractAddress, Amount string, decimalsCache map[xc.ContractAddress]int) (xc.AmountBlockchain, error) {
	decimals, ok := decimalsCache[assetID]
	if !ok {
		var err error
		decimals, err = client.FetchDecimals(ctx, assetID)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"asset_id": assetID,
				"amount":   Amount,
			}).WithError(err).Debug("failed to resolve offer decimals, falling back to zero Amount")
			return xc.NewAmountBlockchainFromUint64(0), nil
		}
		decimalsCache[assetID] = decimals
	}
	blockchainAmount, ok := parseHumanAmountToBlockchain(Amount, int32(decimals))
	if !ok {
		return xc.AmountBlockchain{}, fmt.Errorf("invalid offer Amount %q for asset %q", Amount, assetID)
	}
	return blockchainAmount, nil
}

func isPendingOfferTemplate(templateID *v2.Identifier) bool {
	if templateID == nil {
		return false
	}
	return (templateID.GetModuleName() == "Splice.Wallet.TransferOffer" && templateID.GetEntityName() == "TransferOffer") ||
		(templateID.GetModuleName() == "Utility.Registry.App.V0.Model.Transfer" && templateID.GetEntityName() == "TransferOffer")
}

func isSettlementTemplate(templateID *v2.Identifier) bool {
	if templateID == nil {
		return false
	}
	return templateID.GetModuleName() == "Splice.Wallet.TransferOffer" && templateID.GetEntityName() == "AcceptedTransferOffer"
}

func (client *Client) listOffers(ctx context.Context, args *xclient.OfferArgs, includeTemplate func(*v2.Identifier) bool) ([]*xclient.Offer, error) {
	partyID := string(args.Address())
	if partyID == "" {
		return nil, fmt.Errorf("empty address")
	}
	ledgerEnd, err := client.LedgerClient.GetLedgerEnd(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledger end: %w", err)
	}
	contracts, err := client.LedgerClient.GetActiveContracts(ctx, partyID, ledgerEnd, false)
	if err != nil {
		return nil, fmt.Errorf("failed to query visible offers for party %s: %w", partyID, err)
	}
	filterContract, filterByContract := args.Contract()
	decimalsCache := map[xc.ContractAddress]int{}
	offers := make([]*xclient.Offer, 0, len(contracts))
	for _, contract := range contracts {
		created := contract.GetCreatedEvent()
		if created == nil || !includeTemplate(created.GetTemplateId()) {
			continue
		}
		from, to, assetID, amountText, expiresAt, trackingID, ok := extractLedgerOffer(created)
		if !ok {
			continue
		}
		if from != args.Address() && to != args.Address() {
			continue
		}
		if filterByContract && assetID != filterContract {
			continue
		}
		Amount, err := client.resolveOfferAmount(ctx, assetID, amountText, decimalsCache)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve Amount for offer %s: %w", created.GetContractId(), err)
		}
		offers = append(offers, &xclient.Offer{
			ID:         created.GetContractId(),
			AssetID:    assetID,
			From:       from,
			To:         to,
			Amount:     Amount,
			ExpiresAt:  expiresAt,
			TrackingID: trackingID,
		})
	}
	return offers, nil
}

func (client *Client) ListPendingOffers(ctx context.Context, args *xclient.OfferArgs) ([]*xclient.Offer, error) {
	return client.listOffers(ctx, args, isPendingOfferTemplate)
}

func (client *Client) ListSettlements(ctx context.Context, args *xclient.OfferArgs) ([]*xclient.Settlement, error) {
	offers, err := client.listOffers(ctx, args, isSettlementTemplate)
	if err != nil {
		return nil, err
	}
	settlements := make([]*xclient.Settlement, 0, len(offers))
	for _, offer := range offers {
		settlements = append(settlements, &xclient.Settlement{
			ID:         offer.ID,
			AssetID:    offer.AssetID,
			From:       offer.From,
			To:         offer.To,
			Amount:     offer.Amount,
			ExpiresAt:  offer.ExpiresAt,
			TrackingID: offer.TrackingID,
		})
	}
	return settlements, nil
}

func newCallInputFromPrepareResponse(ledgerEnd int64, activeTime time.Duration, resp *interactive.PrepareSubmissionResponse) *tx_input.CallInput {
	input := tx_input.NewCallInput()
	input.LedgerEnd = ledgerEnd
	input.PreparedTransaction = resp.GetPreparedTransaction()
	input.SubmissionId = NewCommandId()
	input.HashingSchemeVersion = resp.GetHashingSchemeVersion()
	input.DeduplicationWindow = cantonproto.ResolveDeduplicationWindow(activeTime)
	return input
}

func findVisibleActiveContractByID(contracts []*v2.ActiveContract, contractID string) *v2.ActiveContract {
	for _, contract := range contracts {
		created := contract.GetCreatedEvent()
		if created == nil {
			continue
		}
		if created.GetContractId() == contractID {
			return contract
		}
	}
	return nil
}

func (client *Client) prepareWalletOfferAccept(ctx context.Context, partyID string, target *v2.ActiveContract) (*interactive.PrepareSubmissionResponse, error) {
	created := target.GetCreatedEvent()
	if created == nil || created.GetTemplateId() == nil {
		return nil, fmt.Errorf("wallet offer contract is missing created event data")
	}
	commandID := cantonproto.NewCommandID()
	synchronizerID, err := client.resolveSynchronizerID(ctx, partyID, "")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve synchronizer for wallet offer accept: %w", err)
	}
	cmd := buildWalletTransferOfferAcceptCommand(created.GetTemplateId(), created.GetContractId())
	return client.LedgerClient.PrepareSubmissionRequest(ctx, cmd, commandID, partyID, synchronizerID)
}

func (client *Client) prepareTokenOfferAccept(ctx context.Context, partyID string, target *v2.ActiveContract) (*interactive.PrepareSubmissionResponse, error) {
	created := target.GetCreatedEvent()
	if created == nil {
		return nil, fmt.Errorf("token offer contract is missing created event data")
	}
	_, _, assetID, _, _, _, ok := extractLedgerOffer(created)
	if !ok {
		return nil, fmt.Errorf("unsupported token offer contract %s: could not extract transfer fields", created.GetContractId())
	}
	InstrumentAdmin, _, ok := tokenRegistryKey(assetID).Parts()
	if !ok {
		return nil, fmt.Errorf("unsupported token offer contract %s: invalid asset id %q", created.GetContractId(), assetID)
	}
	transferPackageID, err := client.LedgerClient.ResolvePackageIDByName(ctx, "splice-api-token-transfer-instruction-v1")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve token transfer interface package: %w", err)
	}

	tryRegistry := func() (*interactive.PrepareSubmissionResponse, error) {
		registryBaseURL, registryToken, err := client.resolveTokenRegistryBaseURL(ctx, assetID, InstrumentAdmin)
		if err != nil {
			return nil, err
		}
		choiceContext, err := client.LedgerClient.GetTokenTransferInstructionAcceptContextAt(ctx, registryToken, registryBaseURL, created.GetContractId())
		if err != nil {
			return nil, err
		}
		packageMap, err := client.LedgerClient.ListKnownPackageIDsByName(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve package id map for token accept disclosures: %w", err)
		}
		disclosedContracts, registrySynchronizerID, err := tokenDisclosedContractsToProto(choiceContext.DisclosedContracts, packageMap)
		if err != nil {
			return nil, fmt.Errorf("failed to convert token accept disclosures: %w", err)
		}
		cmd, err := buildTokenTransferInstructionAcceptCommand(transferPackageID, created.GetContractId(), choiceContext.ChoiceContextData)
		if err != nil {
			return nil, err
		}
		commandID := cantonproto.NewCommandID()
		synchronizerID, err := client.resolveSynchronizerID(ctx, partyID, registrySynchronizerID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve token accept synchronizer: %w", err)
		}
		prepareReq := cantonproto.NewPrepareRequest(commandID, synchronizerID, []string{partyID}, []string{partyID}, []*v2.Command{cmd}, disclosedContracts)
		return client.LedgerClient.PrepareSubmission(ctx, prepareReq)
	}

	tryFallback := func() (*interactive.PrepareSubmissionResponse, error) {
		cmd, err := buildTokenTransferInstructionAcceptCommand(transferPackageID, created.GetContractId(), map[string]any{"values": map[string]any{}})
		if err != nil {
			return nil, err
		}
		commandID := cantonproto.NewCommandID()
		synchronizerID, err := client.resolveSynchronizerID(ctx, partyID, "")
		if err != nil {
			return nil, fmt.Errorf("failed to resolve token accept synchronizer: %w", err)
		}
		prepareReq := cantonproto.NewPrepareRequest(commandID, synchronizerID, []string{partyID}, []string{partyID}, []*v2.Command{cmd}, nil)
		return client.LedgerClient.PrepareSubmission(ctx, prepareReq)
	}

	prepareResp, err := tryRegistry()
	if err == nil {
		return prepareResp, nil
	}
	client.logger().WithError(err).WithField("contract_id", created.GetContractId()).Warn("registry-backed token accept preparation failed, falling back to ledger-only accept")

	prepareResp, fallbackErr := tryFallback()
	if fallbackErr != nil {
		return nil, fmt.Errorf("failed token offer accept via registry (%v) and fallback (%w)", err, fallbackErr)
	}
	return prepareResp, nil
}

func (client *Client) prepareAcceptedTransferOfferComplete(ctx context.Context, partyID string, acceptedOffer *v2.ActiveContract, visibleContracts []*v2.ActiveContract) (*interactive.PrepareSubmissionResponse, error) {
	created := acceptedOffer.GetCreatedEvent()
	if created == nil || created.GetTemplateId() == nil {
		return nil, fmt.Errorf("settlement contract is missing created event data")
	}

	uiToken, err := client.cantonUIToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire scan token for settlement completion: %w", err)
	}
	amuletRules, err := client.LedgerClient.GetAmuletRules(ctx, uiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch amulet rules: %w", err)
	}
	openMiningRound, issuingMiningRound, err := client.LedgerClient.GetOpenAndIssuingMiningRound(ctx, uiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch mining rounds: %w", err)
	}

	amulets := FilterToAmuletContracts(visibleContracts)
	transferInputs := make([]*v2.Value, 0, len(amulets))
	disclosedContracts := make([]*v2.DisclosedContract, 0, len(amulets)+3)
	for _, contract := range amulets {
		event := contract.GetCreatedEvent()
		if event == nil {
			continue
		}
		if event.GetTemplateId().GetEntityName() == "Amulet" {
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
	}

	amuletRulesTemplateParts := strings.SplitN(amuletRules.AmuletRulesUpdate.Contract.TemplateID, ":", 3)
	if len(amuletRulesTemplateParts) == 3 {
		disclosedContracts = append(disclosedContracts, &v2.DisclosedContract{
			TemplateId: &v2.Identifier{
				PackageId:  amuletRulesTemplateParts[0],
				ModuleName: amuletRulesTemplateParts[1],
				EntityName: amuletRulesTemplateParts[2],
			},
			ContractId:       amuletRules.AmuletRulesUpdate.Contract.ContractID,
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
			ContractId:       openMiningRound.Contract.ContractID,
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

	roundNumber, err := strconv.ParseInt(issuingMiningRound.Contract.Payload.Round.Number, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issuing mining round number: %w", err)
	}

	cmd := buildAcceptedTransferOfferCompleteCommand(
		created.GetTemplateId(),
		created.GetContractId(),
		partyID,
		amuletRules.AmuletRulesUpdate.Contract.ContractID,
		openMiningRound.Contract.ContractID,
		issuingMiningRound.Contract.ContractID,
		roundNumber,
		transferInputs,
	)
	prepareReq := &interactive.PrepareSubmissionRequest{
		UserId:             client.LedgerClient.ValidatorServiceUserID,
		CommandId:          newRegisterCommandId(),
		Commands:           []*v2.Command{cmd},
		ReadAs:             []string{partyID, client.LedgerClient.ValidatorPartyID},
		ActAs:              []string{partyID},
		SynchronizerId:     amuletRules.AmuletRulesUpdate.DomainID,
		DisclosedContracts: disclosedContracts,
		VerboseHashing:     false,
	}
	return client.LedgerClient.PrepareSubmission(ctx, prepareReq)
}

func (client *Client) FetchCallInput(ctx context.Context, call xc.TxCall) (xc.CallTxInput, error) {
	signers := call.SigningAddresses()
	if len(signers) == 0 {
		return nil, fmt.Errorf("no signing address provided for Canton call")
	}
	partyID := string(signers[0])
	contractAddresses := call.ContractAddresses()
	if len(contractAddresses) == 0 || contractAddresses[0] == "" {
		return nil, fmt.Errorf("missing target contract id for Canton call %q", call.GetMethod())
	}
	contractID := string(contractAddresses[0])

	ledgerEnd, err := client.LedgerClient.GetLedgerEnd(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledger end: %w", err)
	}
	contracts, err := client.LedgerClient.GetActiveContracts(ctx, partyID, ledgerEnd, true)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch visible contracts for party %s: %w", partyID, err)
	}
	target := findVisibleActiveContractByID(contracts, contractID)
	if target == nil {
		return nil, fmt.Errorf("contract %s is not visible to caller %s", contractID, partyID)
	}
	created := target.GetCreatedEvent()
	if created == nil || created.GetTemplateId() == nil {
		return nil, fmt.Errorf("contract %s is missing created event data", contractID)
	}

	var prepareResp *interactive.PrepareSubmissionResponse
	switch call.GetMethod() {
	case xccall.OfferAccept:
		templateID := created.GetTemplateId()
		if templateID.GetModuleName() == "Splice.Wallet.TransferOffer" && templateID.GetEntityName() == "TransferOffer" {
			prepareResp, err = client.prepareWalletOfferAccept(ctx, partyID, target)
			if err != nil {
				return nil, err
			}
			break
		}
		if _, _, assetID, _, _, _, ok := extractLedgerOffer(created); ok {
			if tokenRegistryKey(assetID).Valid() {
				prepareResp, err = client.prepareTokenOfferAccept(ctx, partyID, target)
				if err != nil {
					return nil, err
				}
				break
			}
		}
		return nil, fmt.Errorf("unsupported offer accept target %s (%s:%s)", contractID, templateID.GetModuleName(), templateID.GetEntityName())
	case xccall.SettlementComplete:
		templateID := created.GetTemplateId()
		if !isSettlementTemplate(templateID) {
			return nil, fmt.Errorf("unsupported settlement completion target %s (%s:%s)", contractID, templateID.GetModuleName(), templateID.GetEntityName())
		}
		prepareResp, err = client.prepareAcceptedTransferOfferComplete(ctx, partyID, target, contracts)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported Canton call method %q", call.GetMethod())
	}

	return newCallInputFromPrepareResponse(ledgerEnd, client.Asset.TransactionActiveTime, prepareResp), nil
}

func parseRecoveryLookupId(value string) (int64, string, bool) {
	idx := strings.Index(value, "-")
	if idx <= 0 || idx == len(value)-1 {
		return 0, "", false
	}
	beginExclusive, err := strconv.ParseInt(value[:idx], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return beginExclusive, value[idx+1:], true
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	if contract, ok := args.Contract(); ok {
		partyID := string(args.Address())
		if partyID == "" {
			return zero, fmt.Errorf("empty address")
		}
		ledgerEnd, err := client.LedgerClient.GetLedgerEnd(ctx)
		if err != nil {
			return zero, fmt.Errorf("failed to get ledger end: %w", err)
		}
		packageID, err := client.LedgerClient.ResolvePackageIDByName(ctx, "splice-api-token-holding-v1")
		if err != nil {
			return zero, fmt.Errorf("failed to resolve token holding interface package: %w", err)
		}
		contracts, err := client.LedgerClient.GetTokenHoldingContracts(ctx, partyID, ledgerEnd, packageID)
		if err != nil {
			return zero, fmt.Errorf("failed to query token holding contracts for party %s: %w", partyID, err)
		}

		admin, InstrumentID, ok := tokenRegistryKey(contract).Parts()
		if !ok {
			return zero, fmt.Errorf("invalid Canton token contract %q, expected <instrument-admin>#<instrument-id>", contract)
		}
		decimals, ok := args.Decimals()
		if !ok {
			var err error
			decimals, err = client.FetchDecimals(ctx, contract)
			if err != nil {
				return zero, fmt.Errorf("failed to resolve decimals for Canton token contract %q: %w", contract, err)
			}
		}

		totalBalance := xc.NewAmountBlockchainFromUint64(0)
		for _, c := range contracts {
			created := c.GetCreatedEvent()
			Owner, holdingAdmin, holdingID, Amount, ok := ExtractTokenHoldingView(created)
			if !ok || Owner != partyID || holdingAdmin != admin || holdingID != InstrumentID {
				continue
			}
			bal, ok := parseHumanAmountToBlockchain(Amount, int32(decimals))
			if !ok {
				continue
			}
			totalBalance = totalBalance.Add(&bal)
		}
		return totalBalance, nil
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
	ledgerEnd, err := client.LedgerClient.GetLedgerEnd(ctx)
	if err != nil {
		return zero, fmt.Errorf("failed to get ledger end: %w", err)
	}

	contracts, err := client.LedgerClient.GetActiveContracts(ctx, string(address), ledgerEnd, false)
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
	if client.Asset.GetChain().IsChain(contract) {
		return int(client.Asset.GetChain().GetDecimals()), nil
	}
	if assetCfg, ok := client.Asset.FindAdditionalNativeAsset(contract); ok && assetCfg.Decimals > 0 {
		return int(assetCfg.Decimals), nil
	}
	InstrumentAdmin, InstrumentID, ok := tokenRegistryKey(contract).Parts()
	if ok {
		registryBaseURL, registryToken, err := client.resolveTokenRegistryBaseURL(ctx, contract, InstrumentAdmin)
		if err != nil {
			return 0, fmt.Errorf("failed to resolve token registry for Canton token contract %q: %w", contract, err)
		}
		registryInfo, err := client.LedgerClient.GetTokenMetadataRegistryInfoAt(ctx, registryToken, registryBaseURL)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch token metadata registry info for Canton token contract %q: %w", contract, err)
		}
		if registryInfo.AdminID != InstrumentAdmin {
			return 0, fmt.Errorf(
				"token metadata registry admin mismatch for Canton token contract %q: registry admin %q does not match instrument admin %q",
				contract,
				registryInfo.AdminID,
				InstrumentAdmin,
			)
		}
		metadata, err := client.LedgerClient.GetTokenInstrumentMetadataAt(ctx, registryToken, registryBaseURL, InstrumentID)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch token metadata for Canton token contract %q: %w", contract, err)
		}
		if metadata.Decimals < 0 {
			return 0, fmt.Errorf("invalid negative decimals %d for Canton token contract %q", metadata.Decimals, contract)
		}
		return int(metadata.Decimals), nil
	}
	return 0, fmt.Errorf("invalid Canton token contract %q, expected <instrument-admin>#<instrument-id>", contract)
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	ledgerEnd, err := client.LedgerClient.GetLedgerEnd(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledger end: %w", err)
	}
	if ledgerEnd < 0 {
		return nil, fmt.Errorf("invalid negative ledger end offset %d", ledgerEnd)
	}

	height := uint64(ledgerEnd)
	if requestedHeight, ok := args.Height(); ok {
		if requestedHeight > uint64(ledgerEnd) {
			return nil, fmt.Errorf("requested Canton ledger offset %d is after current ledger end %d", requestedHeight, ledgerEnd)
		}
		height = requestedHeight
	}

	return &txinfo.BlockWithTransactions{
		Block: *txinfo.NewBlock(
			client.Asset.GetChain().Chain,
			height,
			fmt.Sprintf("ledger-offset/%d", height),
			time.Time{},
		),
		TransactionIds: []string{},
	}, nil
}

// KeyFingerprintFromAddress extracts the key fingerprint from a Canton party address
func KeyFingerprintFromAddress(addr xc.Address) (string, error) {
	return cantonaddress.FingerprintFromPartyID(addr)
}

var _ xclient.CreateAccountClient = &Client{}

func (client *Client) GetAccountState(ctx context.Context, args *xclient.CreateAccountArgs) (xclient.AccountState, error) {
	partyID := string(args.GetAddress())
	Logger := logrus.WithFields(logrus.Fields{
		"chain":    client.Asset.GetChain().Chain,
		"party_id": partyID,
	})

	exists, err := client.LedgerClient.ExternalPartyExists(ctx, partyID)
	if err != nil {
		Logger.WithError(err).Error("get-account-state: external party registration check failed")
		return "", fmt.Errorf("failed to check external party registration: %w", err)
	}
	if !exists {
		return xclient.AccountInactive, nil
	}

	ledgerEnd, err := client.LedgerClient.GetLedgerEnd(ctx)
	if err != nil {
		Logger.WithError(err).Error("get-account-state: get ledger end failed")
		return "", fmt.Errorf("failed to get ledger end: %w", err)
	}
	contracts, err := client.LedgerClient.GetActiveContracts(ctx, partyID, ledgerEnd, true)
	if err != nil {
		if isPermissionDenied(err) {
			Logger.WithError(err).Info("get-account-state: party exists but contract visibility is not ready yet")
			return xclient.AccountInactive, nil
		}
		Logger.WithError(err).Error("get-account-state: get active contracts failed")
		return "", fmt.Errorf("failed to fetch active contracts: %w", err)
	}
	if client.LedgerClient.HasTransferPreapprovalContract(ctx, contracts) {
		return xclient.AccountActive, nil
	}

	return xclient.AccountInactive, nil
}

// FetchCreateAccountInput fetches all on-chain data required to register a Canton external party
// and advances all registration steps that do not require an explicit external
// signature. If another signed step is needed, it returns the payload for that
// step;
// It will return an AlreadyExists error if the address is already active.
func (client *Client) FetchCreateAccountInput(ctx context.Context, args *xclient.CreateAccountArgs) (xc.CreateAccountTxInput, error) {
	publicKeyBytes := args.GetPublicKey()
	partyID := string(args.GetAddress())
	Logger := logrus.WithFields(logrus.Fields{
		"chain":          client.Asset.GetChain().Chain,
		"party_id":       partyID,
		"public_key_len": len(publicKeyBytes),
	})

	Logger.Info("create-account: checking external party registration")
	exists, err := client.LedgerClient.ExternalPartyExists(ctx, partyID)
	if err != nil {
		Logger.WithError(err).Error("create-account: external party registration check failed")
		return nil, fmt.Errorf("failed to check external party registration: %w", err)
	}
	Logger.WithField("exists", exists).Info("create-account: external party registration check completed")
	if !exists {
		partyHint := hex.EncodeToString(publicKeyBytes)
		signingPubKey := &v2.SigningPublicKey{
			Format:  v2.CryptoKeyFormat_CRYPTO_KEY_FORMAT_RAW,
			KeyData: publicKeyBytes,
			KeySpec: v2.SigningKeySpec_SIGNING_KEY_SPEC_EC_CURVE25519,
		}

		Logger.WithField("party_hint", partyHint).Info("create-account: generating external party topology")
		synchronizerID, err := client.resolveValidatorSynchronizerID(ctx)
		if err != nil {
			Logger.WithError(err).Error("create-account: resolve synchronizer failed")
			return nil, fmt.Errorf("failed to resolve synchronizer for topology generation: %w", err)
		}
		topologyResp, err := client.LedgerClient.GenerateExternalPartyTopology(ctx, &admin.GenerateExternalPartyTopologyRequest{
			Synchronizer: synchronizerID,
			PartyHint:    partyHint,
			PublicKey:    signingPubKey,
		})
		if err != nil {
			Logger.WithError(err).Error("create-account: generate external party topology failed")
			return nil, fmt.Errorf("GenerateExternalPartyTopology failed: %w", err)
		}
		Logger.WithFields(logrus.Fields{
			"topology_tx_count": len(topologyResp.GetTopologyTransactions()),
			"multihash_len":     len(topologyResp.GetMultiHash()),
		}).Info("create-account: generated external party topology")

		input := &tx_input.CreateAccountInput{
			Stage:                tx_input.CreateAccountStageAllocate,
			PartyID:              partyID,
			PublicKeyFingerprint: topologyResp.GetPublicKeyFingerprint(),
			TopologyTransactions: topologyResp.GetTopologyTransactions(),
		}

		if err := input.VerifySignaturePayloads(); err != nil {
			Logger.WithError(err).Error("create-account: allocate-stage input verification failed")
			return nil, fmt.Errorf("hash verification failed after fetch: %w", err)
		}
		Logger.Info("create-account: returning allocate-stage input")
		return input, nil
	}

	Logger.Info("create-account: granting validator service user rights")
	if err := client.LedgerClient.CreateUser(ctx, partyID); err != nil {
		Logger.WithError(err).Error("create-account: grant user rights failed")
		return nil, fmt.Errorf("CreateUser failed: %w", err)
	}
	Logger.Info("create-account: granted validator service user rights")
	Logger.Info("create-account: creating external party setup proposal")
	if err := client.LedgerClient.CreateExternalPartySetupProposal(ctx, partyID); err != nil {
		Logger.WithError(err).Error("create-account: create external party setup proposal failed")
		return nil, fmt.Errorf("CreateExternalPartySetupProposal failed: %w", err)
	}
	Logger.Info("create-account: created external party setup proposal")

	Logger.Info("create-account: fetching ledger end")
	ledgerEnd, err := client.LedgerClient.GetLedgerEnd(ctx)
	if err != nil {
		Logger.WithError(err).Error("create-account: get ledger end failed")
		return nil, fmt.Errorf("failed to get ledger end: %w", err)
	}
	Logger.WithField("ledger_end", ledgerEnd).Info("create-account: fetched ledger end")
	Logger.Info("create-account: fetching active contracts")
	contracts, err := client.LedgerClient.GetActiveContracts(ctx, partyID, ledgerEnd, true)
	if err != nil {
		Logger.WithError(err).Error("create-account: get active contracts failed")
		return nil, fmt.Errorf("failed to fetch active contracts: %w", err)
	}
	Logger.WithField("contract_count", len(contracts)).Info("create-account: fetched active contracts")
	if client.LedgerClient.HasTransferPreapprovalContract(ctx, contracts) {
		return nil, xcerrors.AddressAlreadyActivef("address is already active; transfer preapproval already exists")
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

		cmd := buildExternalPartySetupProposalAcceptCommand(tid, event.GetContractId())
		commandID := cantonproto.NewCommandID()
		Logger.WithFields(logrus.Fields{
			"contract_id": event.GetContractId(),
			"template_id": tid.String(),
			"command_id":  commandID,
		}).Info("create-account: preparing setup proposal accept submission")
		synchronizerID, err := client.resolveSynchronizerID(ctx, partyID, "")
		if err != nil {
			Logger.WithError(err).Error("create-account: resolve accept synchronizer failed")
			return nil, fmt.Errorf("failed to resolve synchronizer for ExternalPartySetupProposal_Accept: %w", err)
		}
		prepareResp, err := client.LedgerClient.PrepareSubmissionRequest(ctx, cmd, commandID, partyID, synchronizerID)
		if err != nil {
			Logger.WithError(err).Error("create-account: prepare setup proposal accept failed")
			return nil, fmt.Errorf("failed to prepare ExternalPartySetupProposal_Accept: %w", err)
		}
		input := &tx_input.CreateAccountInput{
			Stage:                            tx_input.CreateAccountStageAccept,
			PartyID:                          partyID,
			SetupProposalPreparedTransaction: prepareResp.GetPreparedTransaction(),
			SetupProposalHashing:             prepareResp.GetHashingSchemeVersion(),
			SetupProposalSubmissionID:        newRegisterCommandId(),
		}
		if err := input.VerifySignaturePayloads(); err != nil {
			Logger.WithError(err).Error("create-account: accept-stage input verification failed")
			return nil, fmt.Errorf("hash verification failed after fetch: %w", err)
		}
		Logger.WithFields(logrus.Fields{
			"stage":         input.Stage,
			"submission_id": input.SetupProposalSubmissionID,
		}).Info("create-account: returning accept-stage input")
		return input, nil
	}

	Logger.Info("create-account: no further action required")
	return nil, nil
}

func (client *Client) submitCreateAccountTx(ctx context.Context, createAccountTx *cantontx.CreateAccountTx) error {
	if createAccountTx == nil || createAccountTx.Input == nil {
		return fmt.Errorf("create-account tx is nil")
	}
	cantonInput := createAccountTx.Input
	if len(cantonInput.Signature) == 0 {
		return fmt.Errorf("create-account transaction is not signed")
	}
	switch cantonInput.Stage {
	case tx_input.CreateAccountStageAllocate:
		synchronizerID, err := client.resolveValidatorSynchronizerID(ctx)
		if err != nil {
			return fmt.Errorf("failed to resolve synchronizer for external party allocation: %w", err)
		}
		keyFingerprint, err := cantonaddress.FingerprintFromPartyID(xc.Address(createAccountTx.Input.PartyID))
		if err != nil {
			return fmt.Errorf("failed to compute key fingerprint from party ID: %w", err)
		}
		req := cantonproto.NewAllocateExternalPartyRequest(synchronizerID, cantonInput.TopologyTransactions, cantonInput.Signature, keyFingerprint)
		_, err = client.LedgerClient.AllocateExternalParty(ctx, req)
		if err != nil && !isAlreadyExists(err) {
			return fmt.Errorf("AllocateExternalParty failed: %w", err)
		}
		return nil
	case tx_input.CreateAccountStageAccept:
		keyFingerprint, err := KeyFingerprintFromAddress(xc.Address(cantonInput.PartyID))
		if err != nil {
			return fmt.Errorf("failed to determine signing fingerprint for setup proposal accept: %w", err)
		}
		executeReq := cantonproto.NewExecuteSubmissionAndWaitRequest(cantonInput.SetupProposalPreparedTransaction, cantonInput.PartyID, cantonInput.Signature, keyFingerprint, cantonInput.SetupProposalSubmissionID, cantonInput.SetupProposalHashing, client.LedgerClient.DeduplicationWindow)
		_, err = client.LedgerClient.ExecuteSubmissionAndWait(ctx, executeReq)
		if err != nil && !isAlreadyExists(err) {
			return fmt.Errorf("ExternalPartySetupProposal_Accept failed: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported create-account stage %q", cantonInput.Stage)
	}
}

func isPermissionDenied(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "code = PermissionDenied")
}
