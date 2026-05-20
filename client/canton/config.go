package canton

import (
	"fmt"
	"strings"

	"github.com/cordialsys/crosschain/config"
)

// TokenRegistryKey identifies a Canton token registry by token contract in
// <instrument-admin>#<instrument-id> form.
type TokenRegistryKey string

func (key TokenRegistryKey) String() string {
	return string(key)
}

func (key TokenRegistryKey) Parts() (instrumentAdmin string, instrumentID string, ok bool) {
	parts := strings.SplitN(string(key), "#", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (key TokenRegistryKey) InstrumentAdmin() (string, bool) {
	instrumentAdmin, _, ok := key.Parts()
	return instrumentAdmin, ok
}

func (key TokenRegistryKey) InstrumentID() (string, bool) {
	_, instrumentID, ok := key.Parts()
	return instrumentID, ok
}

func (key TokenRegistryKey) Valid() bool {
	_, _, ok := key.Parts()
	return ok
}

type CantonConfig struct {
	// KeycloakURL is the Keycloak base URL used for validator and canton-ui token acquisition.
	KeycloakURL string `yaml:"keycloak_url,omitempty"`
	// KeycloakRealm is the Keycloak realm that issues validator and canton-ui tokens.
	KeycloakRealm string `yaml:"keycloak_realm,omitempty"`
	// RestAPIURL is the validator REST base URL used for validator-admin and public validator endpoints.
	RestAPIURL string `yaml:"rest_api_url,omitempty"`
	// JSONAPIURL is the HTTP JSON Ledger API base URL for the Canton participant.
	JSONAPIURL string `yaml:"json_api_url,omitempty"`
	// ScanProxyURL is the validator-hosted scan proxy endpoint we call from the Canton client.
	ScanProxyURL string `yaml:"scan_proxy_url,omitempty"`
	// ScanAPIURL is the upstream Scan node base URL that the proxy targets on our behalf.
	ScanAPIURL string `yaml:"scan_api_url,omitempty"`
	// LighthouseAPIURL is the Lighthouse explorer API base URL used as tx-info fallback.
	LighthouseAPIURL string `yaml:"lighthouse_api_url,omitempty"`
	// TokenRegistryURLs maps Canton token contracts (<instrument-admin>#<instrument-id>) to registry base URLs.
	TokenRegistryURLs map[TokenRegistryKey]string `yaml:"token_registry_urls,omitempty"`
	// EnableTransferOffers allows native Canton transfers to recipients without TransferPreapproval.
	EnableTransferOffers bool `yaml:"enable_transfer_offers,omitempty"`

	// ValidatorAuth is validator client_credentials auth in id:secret form.
	ValidatorClientID     string        `yaml:"validator_client_id,omitempty"`
	ValidatorClientSecret config.Secret `yaml:"validator_client_secret,omitempty"`

	// Catalyst is used to obtain scan proxy tokens.
	CatalystUsername string        `yaml:"catalyst_username,omitempty"`
	CatalystPassword config.Secret `yaml:"catalyst_password,omitempty"`
}

func (cfg *CantonConfig) IsZero() bool {
	return cfg == nil ||
		(cfg.KeycloakURL == "" &&
			cfg.KeycloakRealm == "" &&
			cfg.RestAPIURL == "" &&
			cfg.JSONAPIURL == "" &&
			cfg.ScanProxyURL == "" &&
			cfg.ScanAPIURL == "" &&
			cfg.LighthouseAPIURL == "" &&
			len(cfg.TokenRegistryURLs) == 0 &&
			!cfg.EnableTransferOffers &&
			cfg.ValidatorClientID == "" &&
			cfg.ValidatorClientSecret == "" &&
			cfg.CatalystUsername == "" &&
			cfg.CatalystPassword == "")
}

func (cfg *CantonConfig) TransferOffersEnabled() bool {
	return cfg != nil && cfg.EnableTransferOffers
}

func (cfg *CantonConfig) Validate() error {
	if cfg == nil {
		return fmt.Errorf("missing canton custom config")
	}

	return nil
}
