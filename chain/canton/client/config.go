package client

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/config"
	"gopkg.in/yaml.v3"
)

type CantonConfig struct {
	KeycloakURL      config.Secret `yaml:"keycloak_url,omitempty"`
	KeycloakRealm    config.Secret `yaml:"keycloak_realm,omitempty"`
	ValidatorAuth    config.Secret `yaml:"validator_auth,omitempty"`
	ValidatorPartyID config.Secret `yaml:"validator_party_id,omitempty"`
	RestAPIURL       config.Secret `yaml:"rest_api_url,omitempty"`
	ScanProxyURL     config.Secret `yaml:"scan_proxy_url,omitempty"`
	ScanAPIURL       config.Secret `yaml:"scan_api_url,omitempty"`
	CantonUIAuth     config.Secret `yaml:"canton_ui_auth,omitempty"`
}

func LoadCantonConfig(chain *xc.ChainConfig) (*CantonConfig, error) {
	cfg := &CantonConfig{}
	if chain != nil && len(chain.CustomConfig) > 0 {
		bz, err := yaml.Marshal(chain.CustomConfig)
		if err != nil {
			return nil, fmt.Errorf("marshal canton custom config: %w", err)
		}
		if err := yaml.Unmarshal(bz, cfg); err != nil {
			return nil, fmt.Errorf("unmarshal canton custom config: %w", err)
		}
	}

	return cfg, nil
}

func (cfg *CantonConfig) Validate() error {
	if cfg.KeycloakURL == "" {
		return fmt.Errorf("missing canton custom config field keycloak_url")
	}
	if cfg.KeycloakRealm == "" {
		return fmt.Errorf("missing canton custom config field keycloak_realm")
	}
	if cfg.ValidatorAuth == "" {
		return fmt.Errorf("missing canton custom config field validator_auth")
	}
	if cfg.ValidatorPartyID == "" {
		return fmt.Errorf("missing canton custom config field validator_party_id")
	}
	if cfg.RestAPIURL == "" {
		return fmt.Errorf("missing canton custom config field rest_api_url")
	}
	if cfg.ScanProxyURL == "" {
		return fmt.Errorf("missing canton custom config field scan_proxy_url")
	}
	if cfg.ScanAPIURL == "" {
		return fmt.Errorf("missing canton custom config field scan_api_url")
	}
	if cfg.CantonUIAuth == "" {
		return fmt.Errorf("missing canton custom config field canton_ui_auth")
	}
	return nil
}
