package client

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Run("missing URL", func(t *testing.T) {
		cfg := &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:  xc.CANTON,
				Driver: xc.DriverCanton,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		}
		_, err := NewClient(cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no URL configured")
	})

	t.Run("missing env var", func(t *testing.T) {
		t.Setenv("CANTON_KEYCLOAK_URL", "")
		t.Setenv("CANTON_KEYCLOAK_REALM", "")
		t.Setenv("CANTON_VALIDATOR_ID", "")
		t.Setenv("CANTON_VALIDATOR_SECRET", "")
		t.Setenv("CANTON_UI_ID", "")
		t.Setenv("CANTON_UI_PASSWORD", "")
		cfg := &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:  xc.CANTON,
				Driver: xc.DriverCanton,
			},
			ChainClientConfig: &xc.ChainClientConfig{
				URL: "https://example.com",
			},
		}
		_, err := NewClient(cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "required environment variable")
	})
}
