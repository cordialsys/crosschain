package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TokenResponse holds the fields from a Keycloak token endpoint response.
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
}

// Client wraps Keycloak admin and token operations.
type Client struct {
	baseURL      string
	realm        string
	clientID     string
	clientSecret string
	httpClient   *http.Client

	mu           sync.Mutex
	adminToken   string
	adminExpires time.Time
}

func NewClient(baseURL, realm, clientID, clientSecret string) *Client {
	return &Client{
		baseURL:      baseURL,
		realm:        realm,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (k *Client) tokenEndpoint() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", k.baseURL, k.realm)
}

func (k *Client) adminEndpoint() string {
	return fmt.Sprintf("%s/admin/realms/%s", k.baseURL, k.realm)
}

// AdminToken returns a cached client_credentials token for Keycloak Admin API calls.
func (k *Client) AdminToken(ctx context.Context) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.adminToken != "" && time.Now().Before(k.adminExpires) {
		return k.adminToken, nil
	}

	resp, err := k.postToken(ctx, url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {k.clientID},
		"client_secret": {k.clientSecret},
	})
	if err != nil {
		return "", err
	}

	k.adminToken = resp.AccessToken
	k.adminExpires = time.Now().Add(time.Duration(resp.ExpiresIn-30) * time.Second)
	return k.adminToken, nil
}

// CreateUser creates a Keycloak user with the given username and password.
// Returns the new user's Keycloak ID. If the user already exists, returns its ID.
func (k *Client) CreateUser(ctx context.Context, username, password string) (string, error) {
	adminToken, err := k.AdminToken(ctx)
	if err != nil {
		return "", fmt.Errorf("getting admin token: %w", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"username": username,
		"enabled":  true,
		"credentials": []map[string]any{
			{"type": "password", "value": password, "temporary": false},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		k.adminEndpoint()+"/users", strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		// User already exists — look it up by username
		logrus.WithFields(logrus.Fields{
			"username": username,
		}).Warn("keycloak: user already exists, fetching ID")
		return k.FindUser(ctx, username)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create user returned %d: %s", resp.StatusCode, b)
	}

	// Keycloak returns the new user URL in the Location header
	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("keycloak did not return Location header after user creation")
	}
	parts := strings.Split(location, "/")
	return parts[len(parts)-1], nil
}

// FindUser returns the Keycloak user ID for the given username.
func (k *Client) FindUser(ctx context.Context, username string) (string, error) {
	adminToken, err := k.AdminToken(ctx)
	if err != nil {
		return "", fmt.Errorf("getting admin token: %w", err)
	}

	endpoint := fmt.Sprintf("%s/users?username=%s&exact=true", k.adminEndpoint(), url.QueryEscape(username))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("find user returned %d: %s", resp.StatusCode, body)
	}

	var users []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &users); err != nil {
		return "", fmt.Errorf("decoding user list: %w", err)
	}
	if len(users) == 0 {
		return "", fmt.Errorf("user %q not found in Keycloak", username)
	}
	return users[0].ID, nil
}

// SetPartyAttribute writes the Canton party ID into the user's canton_party_id attribute.
// The Keycloak protocol mapper then injects this into actAs/readAs on every token.
func (k *Client) SetPartyAttribute(ctx context.Context, userID, partyID string) error {
	adminToken, err := k.AdminToken(ctx)
	if err != nil {
		return fmt.Errorf("getting admin token: %w", err)
	}

	validatorPartyID := os.Getenv("CANTON_VALIDATOR_PARTY_ID")
	if validatorPartyID == "" {
		return fmt.Errorf("required environment variable CANTON_VALIDATOR_PARTY_ID is not set")
	}
	participantAud := "https://daml.com/jwt/aud/participant/" + validatorPartyID
	payload, _ := json.Marshal(map[string]any{
		"attributes": map[string][]string{
			"canton_party_id":        {partyID},
			"canton_participant_aud": {participantAud},
		},
	})

	endpoint := fmt.Sprintf("%s/users/%s", k.adminEndpoint(), userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint,
		strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("set party attribute returned %d: %s", resp.StatusCode, b)
	}
	return nil
}

// AcquireUserToken exchanges username/password for an access token with daml_ledger_api scope.
// The returned token will carry actAs/readAs Canton claims if the protocol mapper is configured.
func (k *Client) AcquireUserToken(ctx context.Context, username, password string) (*TokenResponse, error) {
	return k.postToken(ctx, url.Values{
		"grant_type":    {"password"},
		"client_id":     {k.clientID},
		"client_secret": {k.clientSecret},
		"username":      {username},
		"password":      {password},
		"scope":         {"daml_ledger_api"},
	})
}

func (k *Client) AcquireCantonUiToken(ctx context.Context, username, password string) (*TokenResponse, error) {
	return k.postToken(ctx, url.Values{
		"grant_type": {"password"},
		"client_id":  {"canton-ui"},
		"username":   {username},
		"password":   {password},
	})
}

func (k *Client) postToken(ctx context.Context, form url.Values) (*TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.tokenEndpoint(),
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, body)
	}

	var result TokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}
	if result.AccessToken == "" {
		return nil, fmt.Errorf("empty access token in response")
	}
	return &result, nil
}
