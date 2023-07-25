package crosschain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CrosschainTestSuite struct {
	suite.Suite
	Ctx context.Context
}

func (s *CrosschainTestSuite) SetupTest() {
	s.Ctx = context.Background()
}

func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, new(CrosschainTestSuite))
}

// Client

/*
func (s *CrosschainTestSuite) TestNewClient() {
	require := s.Require()
	client, err := NewClient(&xc.AssetConfig{})
	require.NotNil(client)
	require.EqualError(err, "not implemented")
}

func (s *CrosschainTestSuite) TestFetchTxInput() {
	require := s.Require()
	client, _ := NewClient(&xc.AssetConfig{})
	from := xc.Address("from")
	to := xc.Address("to")
	input, err := client.FetchTxInput(s.Ctx, from, to)
	require.NotNil(input)
	require.EqualError(err, "not implemented")
}

func (s *CrosschainTestSuite) TestSubmitTx() {
	require := s.Require()
	client, _ := NewClient(&xc.AssetConfig{})
	err := client.SubmitTx(s.Ctx, &Tx{})
	require.EqualError(err, "not implemented")
}

func (s *CrosschainTestSuite) TestFetchTxInfo() {
	require := s.Require()
	client, _ := NewClient(&xc.AssetConfig{})
	info, err := client.FetchTxInfo(s.Ctx, xc.TxHash("hash"))
	require.NotNil(info)
	require.EqualError(err, "not implemented")
}
*/
