package builder_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	cantonbuilder "github.com/cordialsys/crosschain/chain/canton/builder"
	"github.com/stretchr/testify/require"
)

func TestSupportsMemo(t *testing.T) {
	t.Parallel()

	builder, err := cantonbuilder.NewTxBuilder(&xc.ChainBaseConfig{
		Chain:  xc.CANTON,
		Driver: xc.DriverCanton,
	})
	require.NoError(t, err)
	require.Equal(t, xc.MemoSupportString, builder.SupportsMemo())
}
