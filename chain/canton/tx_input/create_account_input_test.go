package tx_input

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

func TestCreateAccountInputSerializeRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input *CreateAccountInput
	}{
		{
			name: "allocate",
			input: &CreateAccountInput{
				Stage:                CreateAccountStageAllocate,
				Description:          "allocate",
				PartyID:              "party::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				PublicKeyFingerprint: "1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				TopologyTransactions: [][]byte{{0x01, 0x02}, {0x03, 0x04}},
				Signature:            []byte{0x05, 0x06},
			},
		},
		{
			name:  "accept",
			input: mustLoadLiveCreateAccountAcceptInput(t),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bz, err := tt.input.Serialize()
			require.NoError(t, err)

			parsed, err := ParseCreateAccountInput(bz)
			require.NoError(t, err)
			require.Equal(t, tt.input.Stage, parsed.Stage)
			require.Equal(t, tt.input.Description, parsed.Description)
			require.Equal(t, tt.input.PartyID, parsed.PartyID)
			require.Equal(t, hex.EncodeToString(tt.input.Signature), hex.EncodeToString(parsed.Signature))
			require.Equal(t, hex.EncodeToString(tt.input.SetupProposalHash), hex.EncodeToString(parsed.SetupProposalHash))
			require.Equal(t, tt.input.SetupProposalSubmissionID, parsed.SetupProposalSubmissionID)
			require.Equal(t, tt.input.TopologyTransactions, parsed.TopologyTransactions)
		})
	}
}

func TestCreateAccountInputSetSignatures(t *testing.T) {
	t.Parallel()

	input := &CreateAccountInput{}
	err := input.SetSignatures(&xc.SignatureResponse{Signature: []byte{0x01}})
	require.NoError(t, err)
	require.Equal(t, []byte{0x01}, input.Signature)

	err = input.SetSignatures()
	require.ErrorContains(t, err, "expected 1 signature")
}

func TestCreateAccountInputVerifySignaturePayloadsAllocateStage(t *testing.T) {
	t.Parallel()

	input := &CreateAccountInput{
		Stage:                CreateAccountStageAllocate,
		PartyID:              "party::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PublicKeyFingerprint: "1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		TopologyTransactions: [][]byte{{0x01, 0x02}},
	}

	require.NoError(t, input.VerifySignaturePayloads())
}
