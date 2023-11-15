package cosmos

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/suite"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cosmos/types/evmos/ethermint/crypto/ethsecp256k1"
)

type CrosschainTestSuite struct {
	suite.Suite
	Ctx context.Context
}

func (s *CrosschainTestSuite) SetupTest() {
	s.Ctx = context.Background()
}

func TestCosmos(t *testing.T) {
	suite.Run(t, new(CrosschainTestSuite))
}

func (s *CrosschainTestSuite) TestIsEVMOS() {
	require := s.Require()
	is := isEVMOS(&xc.ChainConfig{Asset: "ETH", Driver: string(xc.DriverEVM)})
	require.False(is)

	is = isEVMOS(&xc.ChainConfig{Asset: "ATOM", Driver: string(xc.DriverCosmos)})
	require.False(is)

	is = isEVMOS(&xc.ChainConfig{Asset: "LUNA", Driver: string(xc.DriverCosmos)})
	require.False(is)

	is = isEVMOS(&xc.ChainConfig{Asset: "XPLA", Driver: string(xc.DriverCosmos)})
	require.False(is)

	is = isEVMOS(&xc.ChainConfig{Asset: "XPLA", Driver: string(xc.DriverCosmosEvmos)})
	require.True(is)
}

func (s *CrosschainTestSuite) TestGetPublicKey() {
	require := s.Require()

	pubKey := getPublicKey(&xc.ChainConfig{Driver: string(xc.DriverCosmos)}, []byte{})
	require.Exactly(&secp256k1.PubKey{Key: []byte{}}, pubKey)

	pubKey = getPublicKey(&xc.ChainConfig{Driver: string(xc.DriverCosmosEvmos)}, []byte{})
	require.Exactly(&ethsecp256k1.PubKey{Key: []byte{}}, pubKey)
}

func (s *CrosschainTestSuite) TestGetSighash() {
	require := s.Require()

	sighash := getSighash(&xc.ChainConfig{Driver: string(xc.DriverCosmos)}, []byte{})
	// echo -n '' | openssl dgst -sha256
	require.Exactly("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", hex.EncodeToString(sighash))

	sighash = getSighash(&xc.ChainConfig{Driver: string(xc.DriverCosmosEvmos)}, []byte{})
	require.Exactly("c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470", hex.EncodeToString(sighash))
}
