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
		PartyID:              "e5c86207770b9fb67d73eb4cb8cd6a5f6a5d6a63c66a5459bd77cca45fda6ede::122079aa518eac66dcd662887155c5c7ee36d3b62e38ed0ded2ddc0c7050460bccc8",
		TopologyTransactions: [][]byte{{0x01, 0x02}},
	}

	require.NoError(t, input.VerifySignaturePayloads())
}
