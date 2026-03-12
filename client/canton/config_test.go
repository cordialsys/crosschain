package canton

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTokenRegistryKeyParts(t *testing.T) {
	key := TokenRegistryKey("cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC")

	admin, instrumentID, ok := key.Parts()
	require.True(t, ok)
	require.Equal(t, "cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f", admin)
	require.Equal(t, "CBTC", instrumentID)
	require.True(t, key.Valid())

	gotAdmin, ok := key.InstrumentAdmin()
	require.True(t, ok)
	require.Equal(t, admin, gotAdmin)

	gotInstrumentID, ok := key.InstrumentID()
	require.True(t, ok)
	require.Equal(t, instrumentID, gotInstrumentID)
}

func TestTokenRegistryKeyPartsRejectsMalformedValues(t *testing.T) {
	for _, key := range []TokenRegistryKey{"", "admin", "#id", "admin#"} {
		admin, instrumentID, ok := key.Parts()
		require.False(t, ok)
		require.Empty(t, admin)
		require.Empty(t, instrumentID)
		require.False(t, key.Valid())
	}
}

func TestCantonConfigTransferOffersEnabled(t *testing.T) {
	var nilCfg *CantonConfig
	require.False(t, nilCfg.TransferOffersEnabled())
	require.False(t, (&CantonConfig{}).TransferOffersEnabled())
	require.True(t, (&CantonConfig{EnableTransferOffers: true}).TransferOffersEnabled())
	require.True(t, (&CantonConfig{}).IsZero())
	require.False(t, (&CantonConfig{EnableTransferOffers: true}).IsZero())
	require.False(t, (&CantonConfig{LighthouseAPIURL: "https://lighthouse.example"}).IsZero())
	require.False(t, (&CantonConfig{JSONAPIURL: "https://json-api.example"}).IsZero())
}
