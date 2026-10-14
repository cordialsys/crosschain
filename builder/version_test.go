package builder

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

func TestVersionComparison(t *testing.T) {
	for _, tt := range []struct {
		reported, minimum string
		want              bool
	}{
		{"", "1.2.3", false},
		{"1.2.3", "v1.2.3", true},
		{"v1.2.3", "1.2.3", true},
		{"1.10.0", "1.9.0", true},
		{"1.2.2", "1.2.3", false},
		{"1.2.3-rc.1", "1.2.3", false},
		{"1.2.3+build.1", "1.2.3", true},
		{"invalid", "1.2.3", false},
	} {
		t.Run(tt.reported+"/"+tt.minimum, func(t *testing.T) {
			args, err := NewTransferArgs(xc.NewChainConfig(xc.NEAR).Base(), "from", "to",
				xc.NewAmountBlockchainFromUint64(1), OptionVersion(tt.reported))
			require.NoError(t, err)
			require.Equal(t, tt.want, args.IsVersionAtLeast(tt.minimum))
		})
	}
}

func TestVersionUnsetAndOverride(t *testing.T) {
	args, err := NewTransferArgs(xc.NewChainConfig(xc.NEAR).Base(), "from", "to", xc.NewAmountBlockchainFromUint64(1))
	require.NoError(t, err)
	_, ok := args.GetVersion()
	require.False(t, ok)
	require.False(t, args.IsVersionAtLeast("1.2.3"))
	args.SetVersion("v1.2.3")
	version, ok := args.GetVersion()
	require.True(t, ok)
	require.Equal(t, "v1.2.3", version)
	require.True(t, args.IsVersionAtLeast("1.2.3"))
	args.SetVersion("")
	require.False(t, args.IsVersionAtLeast("1.2.3"))
}
