package tx_input

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

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
