package signer_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/stretchr/testify/require"
)

func TestNewSigner(t *testing.T) {
	privateKey := "289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032"
	s, err := signer.New(xc.DriverEVM, privateKey, nil, false)
	require.NoError(t, err)
	require.NotNil(t, s)

	privateKey = "12345678"
	_, err = signer.New(xc.DriverEVM, privateKey, nil, false)
	require.Error(t, err)

	mnemonic := "input today bottom quality era above february fiction shift student lawsuit order news pelican unaware firm onion fresh assume lazy draw side joy box"
	_, err = signer.New(xc.DriverCosmos, mnemonic, nil, false)
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
			s, err := signer.New(v.alg, v.pri, nil, false, address.OptionAlgorithm(xc.SignatureType(v.algOverride)))
			require.NoError(t, err)
			bytesMsg, _ := hex.DecodeString(v.msg)
			sig, err := s.Sign(xc.TxDataToSign(bytesMsg))
			require.NoError(t, err)
			require.NotNil(t, sig)
			require.Equal(t, v.sig, hex.EncodeToString(sig))

			pub, err := s.PublicKey()
			require.NoError(t, err)
			require.Equal(t, v.pub, hex.EncodeToString(pub))
		})
	}
}

func TestSchnorrVerify(t *testing.T) {
	// my-taproot-key
	publicKeyHex := "3618bd988e18ea4b3538ccc00572f09e432ad6c2293475eb8e877d310654e876"
	// // CASE: signer did not prehash, but message is hashed (valid)
	// payloadHex := "8f73346ff9ce3c7dac654c625f314393fe3675d712b263739ca6cdc469adeca7"
	// signatureHex := "2c5d4ef59a7083ff6b7a2111d541cd753d7a15b6846c08b8a5a5d92190078455471b4acb588e8e36d47c76e7dea323218e5a4ea6a5f19045271d0f158b205cc9"

	// // CASE: signer does prehash, but message is hashed (not valid)
	// payloadHex := "8f73346ff9ce3c7dac654c625f314393fe3675d712b263739ca6cdc469adeca7"
	// signatureHex := "e05cac5ff224035d6ecef4435653fa621d63ae1a0b44698e9cb504d9202f8e0b9ef1072b10adc6eba8af1aec7ad344cef4101cbdb29e20b8e9ae635d7dacae14"

	// // CASE: signer does prehash, but message is not hashed (valid)
	// payloadHex := "f40a48df4b2a70c8b4924bf2654661ed3d95fd66a313eb87237597c628e4a031f40a48df4b2a70c8b4924bf2654661ed3d95fd66a313eb87237597c628e4a03100000200000000000000075e6d106fe38032f1bcb3f06a6295ebda29acc4bf6087d2522a6ef374e3cc09676bb586e1701a76e8c6fd2e434c0cbe55a099c8d09d7cb8af2aaa4cf430a2a8ab9ee645208dd98d445e4d56b86ba59e92d93c87c893e2237f62324f94ec751e12a3ae445661ce5dee78d0650d33362dec29c4f82af05e7e57fb595bbbacf0ca897a0f4e69e05f30f1fa93887f576f158bc945de8cead3caf22a9aa4201fe0f30000000000"
	// base, err := hex.DecodeString(payloadHex)
	// require.NoError(t, err)
	// hash := sha256.Sum256(base)
	// payloadHex = hex.EncodeToString(hash[:])
	// signatureHex := "8c0909186e79a1c659f20648545d33fc547f2304ab4addadff62a360a3a1491aef5ec4786b5833dcc63e8fdfe7c06cdde35fe218cd9616fc16f959ef8c3c6218"

	// // CASE: prehashed message, signer adjusted to properly accept digest signing (valid)
	// payloadHex := "d1579d6684de6924453e2ba85ee8504ea7fd81812a2dd08e071612e5a08dc33f"
	// signatureHex := "3c29e9b9e570ee15124a271a9ec4ba96d4051128fc7f523931eda3c0f39665ea77ed9fb1f5c3b4f3fbe7e27dcf4b16726924bfa8b4f92946a7edd3fd3c20da71"

	payloadHex := "ef29500dfef3211c718504b561e49e3eac0078caf5431043ce0d37480504dace"
	signatureHex := "cd17fb9a41e3bbb90b98d48cd8ad6caffeb809c4b7afc6eaea834bd813bc666ccab794cc99449961f3a8b9283cae431c775e72ec3ac6123be8daa3bb6a83e551"

	publicKeyBz, err := hex.DecodeString(publicKeyHex)
	require.NoError(t, err)
	payload, err := hex.DecodeString(payloadHex)
	require.NoError(t, err)
	signatureBz, err := hex.DecodeString(signatureHex)
	require.NoError(t, err)

	pubKey, err := schnorr.ParsePubKey(publicKeyBz)
	require.NoError(t, err)
	signature, err := schnorr.ParseSignature(signatureBz)
	require.NoError(t, err)

	// schnorr.
	valid := signature.Verify(payload, pubKey)
	require.True(t, valid, "valid signature")

}
