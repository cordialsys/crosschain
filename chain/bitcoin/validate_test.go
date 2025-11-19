package bitcoin_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		chain     string
		network   string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Bitcoin mainnet - valid addresses
		{
			name:      "BTC mainnet - P2PKH (legacy)",
			chain:     string(xc.BTC),
			network:   "mainnet",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantError: false,
		},
		{
			name:      "BTC mainnet - P2SH",
			chain:     string(xc.BTC),
			network:   "mainnet",
			address:   "3Ai1JZ8pdJb2ksieUV8FsxSNVJCpoPi8W6",
			wantError: false,
		},
		{
			name:      "BTC mainnet - Bech32 (segwit)",
			chain:     string(xc.BTC),
			network:   "mainnet",
			address:   "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq",
			wantError: false,
		},
		{
			name:      "BTC mainnet - Bech32m (taproot)",
			chain:     string(xc.BTC),
			network:   "mainnet",
			address:   "bc1p5d7rjq7g6rdk2yhzks9smlaqtedr4dekq08ge8ztwac72sfr9rusxg3297",
			wantError: false,
		},
		// Bitcoin testnet - valid addresses
		{
			name:      "BTC testnet - P2PKH",
			chain:     string(xc.BTC),
			network:   "testnet",
			address:   "mipcBbFg9gMiCh81Kj8tqqdgoZub1ZJRfn",
			wantError: false,
		},
		{
			name:      "BTC testnet - P2SH",
			chain:     string(xc.BTC),
			network:   "testnet",
			address:   "2MzQwSSnBHWHqSAqtTVQ6v47XtaisrJa1Vc",
			wantError: false,
		},
		{
			name:      "BTC testnet - Bech32",
			chain:     string(xc.BTC),
			network:   "testnet",
			address:   "tb1qw508d6qejxtdg4y5r3zarvary0c5xw7kxpjzsx",
			wantError: false,
		},
		// Dogecoin mainnet - valid addresses
		{
			name:      "DOGE mainnet - P2PKH",
			chain:     string(xc.DOGE),
			network:   "mainnet",
			address:   "DH5yaieqoZN36fDVciNyRueRGvGLR3mr7L",
			wantError: false,
		},
		// Litecoin mainnet - valid addresses
		{
			name:      "LTC mainnet - P2PKH",
			chain:     string(xc.LTC),
			network:   "mainnet",
			address:   "LaMT348PWRnrqeeWArpwQPbuanpXDZGEUz",
			wantError: false,
		},
		{
			name:      "LTC mainnet - P2SH",
			chain:     string(xc.LTC),
			network:   "mainnet",
			address:   "MVcg9uEvtWuP5N6V48EHfEtbz48qR8TKZ9",
			wantError: false,
		},
		// Bitcoin Cash - valid addresses
		{
			name:      "BCH mainnet - P2PKH",
			chain:     string(xc.BCH),
			network:   "mainnet",
			address:   "1BpEi6DfDAUFd7GtittLSdBeYJvcoaVggu",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "BTC mainnet - empty address",
			chain:     string(xc.BTC),
			network:   "mainnet",
			address:   "",
			wantError: true,
			errorMsg:  "invalid bitcoin address",
		},
		{
			name:      "BTC mainnet - invalid checksum",
			chain:     string(xc.BTC),
			network:   "mainnet",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNb",
			wantError: true,
			errorMsg:  "invalid bitcoin address",
		},
		{
			name:      "BTC mainnet - address with space in middle",
			chain:     string(xc.BTC),
			network:   "mainnet",
			address:   "1A1zP1eP5QGefi2DM TfTL5SLmv7DivfNa",
			wantError: true,
		},
		{
			name:      "BTC mainnet - testnet address",
			chain:     string(xc.BTC),
			network:   "mainnet",
			address:   "mipcBbFg9gMiCh81Kj8tqqdgoZub1ZJRfn",
			wantError: true,
			errorMsg:  "invalid bitcoin address",
		},
		{
			name:      "BTC mainnet - malformed address",
			chain:     string(xc.BTC),
			network:   "mainnet",
			address:   "not-a-valid-address",
			wantError: true,
			errorMsg:  "invalid bitcoin address",
		},
		{
			name:      "BTC mainnet - too short",
			chain:     string(xc.BTC),
			network:   "mainnet",
			address:   "1A1z",
			wantError: true,
			errorMsg:  "invalid bitcoin address",
		},
		{
			name:      "BTC mainnet - invalid characters",
			chain:     string(xc.BTC),
			network:   "mainnet",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7Divf0O",
			wantError: true,
			errorMsg:  "invalid bitcoin address",
		},
		{
			name:      "DOGE mainnet - bitcoin address",
			chain:     string(xc.DOGE),
			network:   "mainnet",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantError: true,
			errorMsg:  "invalid bitcoin address",
		},
		{
			name:      "LTC mainnet - bitcoin address",
			chain:     string(xc.LTC),
			network:   "mainnet",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantError: true,
			errorMsg:  "invalid bitcoin address",
		},
		// Unsupported chain
		{
			name:      "unsupported chain",
			chain:     "INVALID",
			network:   "mainnet",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantError: true,
			errorMsg:  "unknown bitcoin chain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain:   xc.NativeAsset(tt.chain),
				Network: tt.network,
			}

			err := bitcoin.ValidateAddress(cfg, tt.address)

			if tt.wantError {
				require.Error(err)
				if tt.errorMsg != "" {
					require.Contains(err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(err)
			}
		})
	}
}
