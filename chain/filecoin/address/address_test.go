package address_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/filecoin/address"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	builder, err := address.NewAddressBuilder(xc.NewChainConfig("", xc.DriverFilecoin))
	require.NotNil(t, builder)
	require.NoError(t, err)
}

func TestGetMainnetAddressFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(
		xc.NewChainConfig("", xc.DriverFilecoin).
			WithNet("mainnet"),
	)

	address, err := builder.GetAddressFromPublicKey([]byte{
		4, 227, 218, 207, 254, 226, 131, 203, 251, 86,
		31, 143, 68, 176, 160, 207, 246, 216, 107, 109,
		107, 114, 189, 15, 87, 193, 90, 238, 233, 101,
		199, 8, 164, 180, 50, 52, 85, 105, 45, 148,
		162, 245, 142, 210, 41, 99, 147, 101, 77, 21,
		141, 199, 25, 28, 166, 44, 199, 173, 46, 123,
		151, 16, 196, 180, 179,
	})
	require.NoError(t, err)
	require.Equal(t, xc.Address("f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q"), address)
}

func TestGetTestnetAddressFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("", xc.DriverFilecoin).WithNet("testnet"))

	address, err := builder.GetAddressFromPublicKey([]byte{
		4, 227, 218, 207, 254, 226, 131, 203, 251, 86,
		31, 143, 68, 176, 160, 207, 246, 216, 107, 109,
		107, 114, 189, 15, 87, 193, 90, 238, 233, 101,
		199, 8, 164, 180, 50, 52, 85, 105, 45, 148,
		162, 245, 142, 210, 41, 99, 147, 101, 77, 21,
		141, 199, 25, 28, 166, 44, 199, 173, 46, 123,
		151, 16, 196, 180, 179,
	})
	require.NoError(t, err)
	require.Equal(t, xc.Address("t13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q"), address)
}

func TestAddressToBytes(t *testing.T) {
	vectors := []struct {
		name     string
		address  string
		expected []byte
		err      string
	}{
		{
			name:     "IdAddress",
			address:  "t0143103",
			expected: []byte{0, 255, 221, 8},
		},
		{
			name:     "SecpkAddress",
			address:  "t1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy",
			expected: []byte{1, 164, 107, 12, 112, 247, 234, 6, 181, 35, 193, 241, 126, 0, 248, 249, 65, 210, 50, 227, 159},
		},
		{
			name:     "ActorAddress",
			address:  "f2kbv57glniayy75fausbk7cc3xvrsb2bgvcqscwy",
			expected: []byte{2, 80, 107, 223, 153, 109, 64, 49, 143, 244, 160, 164, 130, 175, 136, 91, 189, 99, 32, 232, 38},
		},
		{
			name:     "BlsAddress",
			address:  "t3vvmn62lofvhjd2ugzca6sof2j2ubwok6cj4xxbfzz4yuxfkgobpihhd2thlanmsh3w2ptld2gqkn2jvlss4a",
			expected: []byte{3, 173, 88, 223, 105, 110, 45, 78, 145, 234, 134, 200, 129, 233, 56, 186, 78, 168, 27, 57, 94, 18, 121, 123, 132, 185, 207, 49, 75, 149, 70, 112, 94, 131, 156, 122, 153, 214, 6, 178, 71, 221, 180, 249, 172, 122, 52, 20, 221},
		},
		{
			name:    "DelegatedAddress",
			address: "t410fxgo7645dlyjza2s5ft67pidjhxc5qzeqsspyzjq",
			err:     "unsupported protocol",
		},
	}

	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			bytes, err := address.AddressToBytes(v.address)
			if v.err != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, v.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, v.expected, bytes)
			}
		})
	}
}
