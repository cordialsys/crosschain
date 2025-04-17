package signer_test

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/stretchr/testify/require"
)

func TestNewSigner(t *testing.T) {
	privateKey := "289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032"
	s, err := signer.New(xc.DriverEVM, privateKey, nil)
	require.NoError(t, err)
	require.NotNil(t, s)

	privateKey = "12345678"
	_, err = signer.New(xc.DriverEVM, privateKey, nil)
	require.Error(t, err)

	mnemonic := "input today bottom quality era above february fiction shift student lawsuit order news pelican unaware firm onion fresh assume lazy draw side joy box"
	_, err = signer.New(xc.DriverCosmos, mnemonic, nil)
	require.NoError(t, err)
}

func TestSign(t *testing.T) {

	vectors := []struct {
		alg         xc.Driver
		pri         string
		pub         string
		msg         string
		sig         string
		algOverride string
	}{
		{
			alg: xc.DriverEVM,
			pri: "289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032",
			pub: "047db227d7094ce215c3a0f57e1bcc732551fe351f94249471934567e0f5dc1bf795962b8cccb87a2eb56b29fbe37d614e2f4c3c45b789ae4f1f51f4cb21972ffd",
			msg: "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470", // keccak256("")
			sig: "b415397b439cc1eaab587a70717499b56b6cbe63037c241b2eaca2e833a6da097002b11c9611964e97212c82eab9613531f40e065d4d32e32ef31d68fedd977501",
		},
		{
			alg: xc.DriverEVM,
			pri: "289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032",
			pub: "047db227d7094ce215c3a0f57e1bcc732551fe351f94249471934567e0f5dc1bf795962b8cccb87a2eb56b29fbe37d614e2f4c3c45b789ae4f1f51f4cb21972ffd",
			msg: "41b1a0649752af1b28b3dc29a1556eee781e4a4c3a1f7f53f90fa834de098c4d", // keccak256("foo")
			sig: "d155e94305af7e07dd8c32873e5c03cb95c9e05960ef85be9c07f671da58c73718c19adc397a211aa9e87e519e2038c5a3b658618db335f74f800b8e0cfeef4401",
		},
		{
			alg: xc.DriverCosmos,
			pri: "9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60",
			pub: "028db55b05db86c0b1786ca49f095d76344c9e6056b2f02701a7e7f3c20aabfd91",
			msg: "41b1a0649752af1b28b3dc29a1556eee781e4a4c3a1f7f53f90fa834de098c4d",
			sig: "1ab02f2e814644cce4f29b934e234d6c6dfbda13653d58863b90ea6790ee4e8f6d626682ad53be99a1d31474c9cdd0bb773595b72a3a11a10d4ab1bd0654dcff01",
		},
		{
			alg: xc.DriverSolana,
			pri: "9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a",
			pub: "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a",
			msg: "",
			sig: "e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e065224901555fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b",
		},
		{
			alg: xc.DriverSolana,
			pri: "940c89fe40a81dafbdb2416d14ae469119869744410c3303bfaa0241dac57800a2eb8c0501e30bae0cf842d2bde8dec7386f6b7fc3981b8c57c9792bb94cf2dd",
			pub: "a2eb8c0501e30bae0cf842d2bde8dec7386f6b7fc3981b8c57c9792bb94cf2dd",
			msg: "b87d3813e03f58cf19fd0b6395",
			sig: "d8bb64aad8c9955a115a793addd24f7f2b077648714f49c4694ec995b330d09d640df310f447fd7b6cb5c14f9fe9f490bcf8cfadbfd2169c8ac20d3b8af49a0c",
		},
		{
			alg:         xc.DriverBitcoin,
			pri:         "1d25811b76f43c86d59d757622773b2969ee71270ea810a42deda024e0cf896a",
			pub:         "03e3dacffee283cbfb561f8f44b0a0cff6d86b6d6b72bd0f57c15aeee965c708a4",
			msg:         "d7d9a2283d9899c96550a848c7ecd6e8a3094780a08c9760730399cb3d594d61",
			sig:         "311832a81c1ff59db4adafedcfad97bc4352d427557954629c81c00c58b25ac5f1e0ff1fa81e5e104ad4a7f470c2931e7903c0d8733d9c846186112579fc7103",
			algOverride: "schnorr",
		},
		{
			alg: xc.DriverBitcoin,
			pri: "1d25811b76f43c86d59d757622773b2969ee71270ea810a42deda024e0cf896a",
			pub: "03e3dacffee283cbfb561f8f44b0a0cff6d86b6d6b72bd0f57c15aeee965c708a4",
			msg: "c6325ffd8690ecdb28e4d655707e3e79e2ab51087e9611555a7ed0064eee285f",
			sig: "fb04cee56283d000ad37232a66b0b98e2acc903b89cbf11b1edb32d51029ee5a74b332ca5b347ff86832daae64fb685d656b13c5b276253bf10bc4eaacfcab7001",
		},
	}

	for _, v := range vectors {
		t.Run(fmt.Sprintf("TestSign-%s-%s", v.alg, v.algOverride), func(t *testing.T) {
			s, err := signer.New(v.alg, v.pri, nil, address.OptionAlgorithm(xc.SignatureType(v.algOverride)))
			require.NoError(t, err)
			bytesMsg, _ := hex.DecodeString(v.msg)
			sig, err := s.Sign(&xc.SignatureRequest{
				Payload: bytesMsg,
			})
			require.NoError(t, err)
			require.NotNil(t, sig)
			require.Equal(t, v.sig, hex.EncodeToString(sig.Signature))

			pub, err := s.PublicKey()
			require.NoError(t, err)
			require.Equal(t, v.pub, hex.EncodeToString(pub))
		})
	}
}

// mac computes a + (b * c) + carry, returning the result and the new carry over.
func mac(a, b, c, carry uint64) (uint64, uint64) {
	// Create big.Int instances for the inputs
	bigA := new(big.Int).SetUint64(a)
	bigB := new(big.Int).SetUint64(b)
	bigC := new(big.Int).SetUint64(c)
	bigCarry := new(big.Int).SetUint64(carry)

	// Perform the multiplication and addition
	product := new(big.Int).Mul(bigB, bigC)
	sum := new(big.Int).Add(bigA, product)
	sum.Add(sum, bigCarry)

	// Extract the lower 64 bits and the higher 64 bits
	lower64 := sum.Uint64()
	upper64 := new(big.Int).Rsh(sum, 64).Uint64()

	return lower64, upper64
}

func TestMacNoCarry(t *testing.T) {
	result, carry := mac(1, 2, 3, 0)
	if result != 7 || carry != 0 {
		t.Errorf("Expected (7, 0), got (%d, %d)", result, carry)
	}
}

func TestMacWithCarry(t *testing.T) {
	result, carry := mac(1, 2, 3, 4)
	if result != 11 || carry != 0 {
		t.Errorf("Expected (11, 0), got (%d, %d)", result, carry)
	}
}

func TestMacLargeValues(t *testing.T) {
	result, carry := mac(^uint64(0), ^uint64(0), ^uint64(0), ^uint64(0))
	if result != 18446744073709551615 || carry != 18446744073709551615 {
		t.Errorf("Expected (18446744073709551615, 18446744073709551615), got (%d, %d)", result, carry)
	}
}

func TestMacOverflow(t *testing.T) {
	result, carry := mac(^uint64(0), ^uint64(0), 2, 0)
	if result != 18446744073709551613 || carry != 2 {
		t.Errorf("Expected (18446744073709551614, 18446744073709551615), got (%d, %d)", result, carry)
	}
}

func TestMacZeroValues(t *testing.T) {
	result, carry := mac(0, 0, 0, 0)
	if result != 0 || carry != 0 {
		t.Errorf("Expected (0, 0), got (%d, %d)", result, carry)
	}
}

// adc computes a + b + carry, returning the result and the new carry over.
func adc(a, b, carry uint64) (uint64, uint64) {
	bigA := new(big.Int).SetUint64(a)
	bigB := new(big.Int).SetUint64(b)
	bigCarry := new(big.Int).SetUint64(carry)

	sum := new(big.Int).Add(bigA, bigB)
	sum.Add(sum, bigCarry)

	result := sum.Uint64()
	carryOut := new(big.Int).Rsh(sum, 64).Uint64()

	return result, carryOut
}

func TestAdcNoCarry(t *testing.T) {
	result, carry := adc(1, 2, 0)
	if result != 3 || carry != 0 {
		t.Errorf("adc(1, 2, 0) = (%d, %d); want (3, 0)", result, carry)
	}
}

func TestAdcWithCarry(t *testing.T) {
	result, carry := adc(^uint64(0), 1, 0)
	if result != 0 || carry != 1 {
		t.Errorf("adc(^uint64(0), 1, 0) = (%d, %d); want (0, 1)", result, carry)
	}
}

func TestAdcWithInitialCarry(t *testing.T) {
	result, carry := adc(1, 1, 1)
	if result != 3 || carry != 0 {
		t.Errorf("adc(1, 1, 1) = (%d, %d); want (3, 0)", result, carry)
	}
}

func TestAdcWithAllMaxValues(t *testing.T) {
	result, carry := adc(^uint64(0), ^uint64(0), ^uint64(0))
	if result != ^uint64(0)-2 || carry != 2 {
		t.Errorf("adc(^uint64(0), ^uint64(0), ^uint64(0)) = (%d, %d); want (%d, 2)", result, carry, ^uint64(0)-2)
	}
}
