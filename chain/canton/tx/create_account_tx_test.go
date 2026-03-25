package tx

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	"github.com/stretchr/testify/require"
)

func TestCreateAccountTxRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input *tx_input.CreateAccountInput
	}{
		{
			name: "allocate",
			input: &tx_input.CreateAccountInput{
				Stage:                tx_input.CreateAccountStageAllocate,
				PartyID:              "party::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				PublicKeyFingerprint: "1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				TopologyTransactions: [][]byte{{0x01, 0x02}},
			},
		},
		{
			name:  "accept",
			input: loadLiveAcceptInput(t),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			args, err := xcbuilder.NewCreateAccountArgs(xc.CANTON, xc.Address(tt.input.PartyID), []byte{0x01, 0x02})
			require.NoError(t, err)

			tx, err := NewCreateAccountTx(args, tt.input)
			require.NoError(t, err)

			sighashes, err := tx.Sighashes()
			require.NoError(t, err)
			require.Len(t, sighashes, 1)
			if tt.input.Stage == tx_input.CreateAccountStageAllocate {
				expectedHash, err := tx_input.ComputeTopologyMultiHash(tt.input.TopologyTransactions)
				require.NoError(t, err)
				require.Equal(t, expectedHash, sighashes[0].Payload)
			} else {
				require.Equal(t, tt.input.SetupProposalHash, sighashes[0].Payload)
			}

			unsigned, err := tx.Serialize()
			require.NoError(t, err)
			require.Len(t, unsigned, 8)

			metadataBz, ok, err := tx.GetMetadata()
			require.NoError(t, err)
			require.True(t, ok)
			metadata, err := ParseMetadata(metadataBz)
			require.NoError(t, err)

			parsedUnsigned, err := ParseCreateAccountTxWithMetadata(unsigned, metadata)
			require.NoError(t, err)
			require.Equal(t, tt.input.Stage, parsedUnsigned.Input.Stage)

			err = tx.SetSignatures(&xc.SignatureResponse{Signature: []byte{0xde, 0xad}})
			require.NoError(t, err)

			signed, err := tx.Serialize()
			require.NoError(t, err)

			parsedSigned, err := ParseCreateAccountTxWithMetadata(signed, metadata)
			require.NoError(t, err)
			require.Equal(t, "dead", hex.EncodeToString(parsedSigned.Input.Signature))
			require.NotEmpty(t, tx.Hash())
		})
	}
}

func loadLiveAcceptInput(t *testing.T) *tx_input.CreateAccountInput {
	t.Helper()

	data, err := os.ReadFile("../tx_input/testdata/live_create_account_accept.json")
	require.NoError(t, err)

	var fixture struct {
		CreateAccountInput string `json:"create_account_input"`
	}
	require.NoError(t, json.Unmarshal(data, &fixture))

	encoded, err := hex.DecodeString(fixture.CreateAccountInput)
	require.NoError(t, err)

	input, err := tx_input.ParseCreateAccountInput(encoded)
	require.NoError(t, err)
	return input
}
