package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/admin"
)

type CantonTrafficInspection struct {
	ValidatorPartyID            string                     `json:"validator_party_id,omitempty"`
	ValidatorServiceUserID      string                     `json:"validator_service_user_id,omitempty"`
	ParticipantID               string                     `json:"participant_id,omitempty"`
	ParticipantIDError          string                     `json:"participant_id_error,omitempty"`
	LedgerEnd                   *int64                     `json:"ledger_end,omitempty"`
	LedgerEndError              string                     `json:"ledger_end_error,omitempty"`
	TrafficStatus               *CantonTrafficStatusReport `json:"traffic_status,omitempty"`
	TrafficStatusError          string                     `json:"traffic_status_error,omitempty"`
	ValidatorOperatorBalance    *CantonTrafficBalance      `json:"validator_operator_balance,omitempty"`
	ValidatorOperatorBalanceErr string                     `json:"validator_operator_balance_error,omitempty"`
	ValidatorAPI                *CantonHTTPEndpointStatus  `json:"validator_api,omitempty"`
	ValidatorAPIError           string                     `json:"validator_api_error,omitempty"`
	AmuletRules                 *CantonAmuletRulesSnapshot `json:"amulet_rules,omitempty"`
	AmuletRulesError            string                     `json:"amulet_rules_error,omitempty"`
	TopupConfig                 CantonTopupConfigStatus    `json:"topup_config"`
	Endpoints                   CantonTrafficEndpoints     `json:"endpoints"`
	Notes                       []string                   `json:"notes,omitempty"`
}

type CantonTrafficBalance struct {
	Address         string `json:"address"`
	BlockchainUnits string `json:"blockchain_units"`
	Human           string `json:"human"`
	Decimals        int32  `json:"decimals"`
}

type CantonHTTPEndpointStatus struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Body       string `json:"body,omitempty"`
}

type CantonAmuletRulesSnapshot struct {
	ContractID string `json:"contract_id,omitempty"`
	TemplateID string `json:"template_id,omitempty"`
	DomainID   string `json:"domain_id,omitempty"`
	DSO        string `json:"dso,omitempty"`
	Fees       any    `json:"fees,omitempty"`
}

type CantonTrafficStatusReport struct {
	DomainID  string               `json:"domain_id"`
	MemberID  string               `json:"member_id"`
	Status    CantonTrafficStatus  `json:"status"`
	Available CantonTrafficBalance `json:"available"`
}

type CantonTrafficStatus struct {
	Actual CantonTrafficActual `json:"actual"`
	Target CantonTrafficTarget `json:"target"`
}

type CantonTrafficActual struct {
	TotalConsumed uint64 `json:"total_consumed"`
	TotalLimit    uint64 `json:"total_limit"`
}

type CantonTrafficTarget struct {
	TotalPurchased uint64 `json:"total_purchased"`
}

type trafficStatusResponse struct {
	TrafficStatus CantonTrafficStatus `json:"traffic_status"`
}

type CantonTopupConfigStatus struct {
	Queryable bool   `json:"queryable"`
	Message   string `json:"message"`
}

type CantonTrafficEndpoints struct {
	RestAPIURL       string `json:"rest_api_url,omitempty"`
	ScanProxyURL     string `json:"scan_proxy_url,omitempty"`
	ScanAPIURL       string `json:"scan_api_url,omitempty"`
	LighthouseAPIURL string `json:"lighthouse_api_url,omitempty"`
	MetricsHint      string `json:"metrics_hint,omitempty"`
}

func (client *Client) InspectTraffic(ctx context.Context) (*CantonTrafficInspection, error) {
	if client == nil || client.LedgerClient == nil {
		return nil, fmt.Errorf("canton client is not initialized")
	}

	result := &CantonTrafficInspection{
		ValidatorPartyID:       client.LedgerClient.ValidatorPartyID,
		ValidatorServiceUserID: client.LedgerClient.ValidatorServiceUserID,
		TopupConfig: CantonTopupConfigStatus{
			Queryable: false,
			Message:   "traffic top-up settings are validator deployment configuration; they are not exposed by the Ledger API or wallet API",
		},
		Endpoints: CantonTrafficEndpoints{
			RestAPIURL:       client.LedgerClient.RestAPIURL,
			ScanProxyURL:     client.LedgerClient.ScanProxyURL,
			ScanAPIURL:       client.LedgerClient.ScanAPIURL,
			LighthouseAPIURL: client.LedgerClient.LighthouseAPIURL,
			MetricsHint:      "Splice apps expose Prometheus metrics on port 10013 at /metrics if the host exposes it",
		},
		Notes: []string{
			"traffic balance is per participant; all parties hosted by this validator share it",
			"the validator operator balance is a useful top-up prerequisite, but it is not the current traffic balance",
		},
	}

	if ledgerEnd, err := client.LedgerClient.GetLedgerEnd(ctx); err != nil {
		result.LedgerEndError = err.Error()
	} else {
		result.LedgerEnd = &ledgerEnd
	}

	if status, err := client.LedgerClient.ValidatorReady(ctx); err != nil {
		result.ValidatorAPIError = err.Error()
	} else {
		result.ValidatorAPI = status
	}

	if participantID, err := client.LedgerClient.GetParticipantID(ctx); err != nil {
		result.ParticipantIDError = err.Error()
	} else {
		result.ParticipantID = participantID
	}

	if result.ValidatorPartyID != "" {
		balance, err := client.FetchNativeBalance(ctx, xc.Address(result.ValidatorPartyID))
		if err != nil {
			result.ValidatorOperatorBalanceErr = err.Error()
		} else {
			decimals := client.Asset.GetChain().Decimals
			result.ValidatorOperatorBalance = &CantonTrafficBalance{
				Address:         result.ValidatorPartyID,
				BlockchainUnits: balance.String(),
				Human:           balance.ToHuman(decimals).String(),
				Decimals:        decimals,
			}
		}
	}

	uiToken, err := client.cantonUIToken(ctx)
	if err != nil {
		result.AmuletRulesError = err.Error()
		return result, nil
	}
	rawRules, err := client.LedgerClient.GetAmuletRulesRaw(ctx, uiToken)
	if err != nil {
		result.AmuletRulesError = err.Error()
		return result, nil
	}
	var amuletRules AmuletRules
	if err := unmarshalMap(rawRules, &amuletRules); err != nil {
		result.AmuletRulesError = err.Error()
		return result, nil
	}

	result.AmuletRules = &CantonAmuletRulesSnapshot{
		ContractID: amuletRules.AmuletRulesUpdate.Contract.ContractID,
		TemplateID: amuletRules.AmuletRulesUpdate.Contract.TemplateID,
		DomainID:   amuletRules.AmuletRulesUpdate.DomainID,
		DSO:        amuletRules.AmuletRulesUpdate.Contract.Payload.DSO,
		Fees:       nestedValue(rawRules, "amulet_rules_update", "contract", "payload", "configSchedule", "initialValue", "decentralizedSynchronizer", "fees"),
	}
	if result.ParticipantID != "" && result.AmuletRules.DomainID != "" {
		trafficStatus, err := client.LedgerClient.GetTrafficStatus(ctx, uiToken, result.AmuletRules.DomainID, result.ParticipantID)
		if err != nil {
			result.TrafficStatusError = err.Error()
		} else {
			result.TrafficStatus = trafficStatus
		}
	}
	return result, nil
}

func (c *GrpcLedgerClient) GetAmuletRulesRaw(ctx context.Context, token string) (map[string]any, error) {
	var result map[string]any
	if err := c.doScanProxyRequest(ctx, token, "/api/scan/v0/amulet-rules", map[string]any{}, &result); err != nil {
		return nil, fmt.Errorf("fetching amulet rules: %w", err)
	}
	return result, nil
}

func (c *GrpcLedgerClient) ValidatorReady(ctx context.Context) (*CantonHTTPEndpointStatus, error) {
	if c.RestAPIURL == "" {
		return nil, fmt.Errorf("validator REST API URL is not configured")
	}
	target := strings.TrimRight(c.RestAPIURL, "/") + "/api/validator/readyz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("creating validator readiness request: %w", err)
	}
	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing validator readiness request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, fmt.Errorf("reading validator readiness response: %w", err)
	}
	return &CantonHTTPEndpointStatus{
		URL:        target,
		StatusCode: resp.StatusCode,
		Body:       strings.TrimSpace(string(body)),
	}, nil
}

func (c *GrpcLedgerClient) GetParticipantID(ctx context.Context) (string, error) {
	resp, err := c.AdminClient.GetParticipantId(c.authCtx(ctx), &admin.GetParticipantIdRequest{})
	if err != nil {
		return "", fmt.Errorf("get participant id: %w", err)
	}
	if resp.GetParticipantId() == "" {
		return "", fmt.Errorf("participant id response is empty")
	}
	return resp.GetParticipantId(), nil
}

func (c *GrpcLedgerClient) GetTrafficStatus(ctx context.Context, token string, domainID string, participantID string) (*CantonTrafficStatusReport, error) {
	if domainID == "" {
		return nil, fmt.Errorf("empty domain id")
	}
	if participantID == "" {
		return nil, fmt.Errorf("empty participant id")
	}
	memberID := participantID
	if !strings.HasPrefix(memberID, "PAR::") && !strings.HasPrefix(memberID, "MED::") {
		memberID = "PAR::" + memberID
	}

	var response trafficStatusResponse
	path := "/api/scan/v0/domains/" + url.PathEscape(domainID) + "/members/" + url.PathEscape(memberID) + "/traffic-status"
	err := c.doScanProxyRequestWithMethod(ctx, token, http.MethodGet, path, nil, &response)
	if err != nil {
		fallbackPath := "/v0/domains/" + url.PathEscape(domainID) + "/members/" + url.PathEscape(memberID) + "/traffic-status"
		fallbackErr := c.doScanProxyRequestWithMethod(ctx, token, http.MethodGet, fallbackPath, nil, &response)
		if fallbackErr != nil {
			return nil, fmt.Errorf("fetching traffic status via %s (%v) and %s (%w)", path, err, fallbackPath, fallbackErr)
		}
	}

	available := uint64(0)
	if response.TrafficStatus.Actual.TotalLimit > response.TrafficStatus.Actual.TotalConsumed {
		available = response.TrafficStatus.Actual.TotalLimit - response.TrafficStatus.Actual.TotalConsumed
	}
	return &CantonTrafficStatusReport{
		DomainID: domainID,
		MemberID: memberID,
		Status:   response.TrafficStatus,
		Available: CantonTrafficBalance{
			Address:         memberID,
			BlockchainUnits: fmt.Sprintf("%d", available),
			Human:           fmt.Sprintf("%d", available),
			Decimals:        0,
		},
	}, nil
}

func nestedValue(value any, path ...string) any {
	current := value
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[key]
	}
	return normalizeJSONValue(current)
}

func normalizeJSONValue(value any) any {
	bz, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var normalized any
	if err := json.Unmarshal(bz, &normalized); err != nil {
		return value
	}
	return normalized
}

func unmarshalMap(value map[string]any, out any) error {
	bz, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal raw amulet rules: %w", err)
	}
	if err := json.Unmarshal(bz, out); err != nil {
		return fmt.Errorf("decode raw amulet rules: %w", err)
	}
	return nil
}
