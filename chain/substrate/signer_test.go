package substrate

import (
	"encoding/hex"

	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	xc "github.com/cordialsys/crosschain"
)

func (s *CrosschainTestSuite) TestNewSigner() {
	require := s.Require()
	signer, err := NewSigner(&xc.NativeAssetConfig{})
	require.Nil(err)
	require.NotNil(signer)
}

func (s *CrosschainTestSuite) TestImportPrivateKey() {
	require := s.Require()
	signer, _ := NewSigner(&xc.NativeAssetConfig{})
	key, err := signer.ImportPrivateKey("0x0931ee5849b18ce7699982d3222b6b861e28336462659e709f93e9d903986da7")
	require.Nil(err)
	require.Equal(hex.EncodeToString(key), "0931ee5849b18ce7699982d3222b6b861e28336462659e709f93e9d903986da7")
}

func (s *CrosschainTestSuite) TestSign() {
	// SR25519 signatures are nondeterministic
	require := s.Require()
	vectors := []struct {
		pri string
		msg string
	}{
		{
			"0931ee5849b18ce7699982d3222b6b861e28336462659e709f93e9d903986da7",
			"d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a",
		},
		{
			"a4cdb9eaaac309b8cf2974001cccd3c4557f1abfb51359ccbd1d6ab87352ebac",
			"a2eb8c0501e30bae0cf842d2bde8dec7386f6b7fc3981b8c57c9792bb94cf2dd",
		},
	}
	for _, v := range vectors {
		signer, _ := NewSigner(&xc.NativeAssetConfig{})
		bytesPri, _ := hex.DecodeString(v.pri)
		bytesMsg, _ := hex.DecodeString(v.msg)
		sig, err := signer.Sign(xc.PrivateKey(bytesPri), xc.TxDataToSign(bytesMsg))
		require.Nil(err)
		require.NotNil(sig)
		ok, err := signature.Verify(bytesMsg, sig, hex.EncodeToString(bytesPri))
		require.Nil(err)
		require.True(ok)

		ok, err = signature.Verify(bytesMsg, sig, hex.EncodeToString(bytesMsg))
		require.Nil(err)
		require.False(ok)
	}
}
