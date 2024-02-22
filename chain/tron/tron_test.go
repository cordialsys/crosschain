package tron

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

// TxBuilder

// func (s *CrosschainTestSuite) TestNewTxBuilder() {
// 	require := s.Require()
// 	builder, err := NewTxBuilder(&xc.ChainConfig{})
// 	require.NotNil(builder)
// 	require.EqualError(err, "not implemented")
// }

// func (s *CrosschainTestSuite) TestNewNativeTransfer() {
// 	require := s.Require()
// 	builder, _ := NewTxBuilder(&xc.ChainConfig{})
// 	from := xc.Address("from")
// 	to := xc.Address("to")
// 	amount := xc.AmountBlockchain{}
// 	input := TxInput{}
// 	tf, err := builder.(xc.TxTokenBuilder).NewNativeTransfer(from, to, amount, input)
// 	require.Nil(tf)
// 	require.EqualError(err, "not implemented")
// }

// func (s *CrosschainTestSuite) TestNewTokenTransfer() {
// 	require := s.Require()
// 	builder, _ := NewTxBuilder(&xc.ChainConfig{})
// 	from := xc.Address("from")
// 	to := xc.Address("to")
// 	amount := xc.AmountBlockchain{}
// 	input := TxInput{}
// 	tf, err := builder.(xc.TxTokenBuilder).NewTokenTransfer(from, to, amount, input)
// 	require.Nil(tf)
// 	require.EqualError(err, "not implemented")
// }

// func (s *CrosschainTestSuite) TestNewTransfer() {
// 	require := s.Require()
// 	builder, _ := NewTxBuilder(&xc.ChainConfig{})
// 	from := xc.Address("from")
// 	to := xc.Address("to")
// 	amount := xc.AmountBlockchain{}
// 	input := TxInput{}
// 	tf, err := builder.NewTransfer(from, to, amount, input)
// 	require.Nil(tf)
// 	require.EqualError(err, "not implemented")
// }

// // Client

// func (s *CrosschainTestSuite) TestNewClient() {
// 	require := s.Require()
// 	client, err := NewClient(&xc.ChainConfig{})
// 	require.NotNil(client)
// 	require.EqualError(err, "not implemented")
// }

// func (s *CrosschainTestSuite) TestFetchTxInput() {
// 	require := s.Require()
// 	client, _ := NewClient(&xc.ChainConfig{})
// 	from := xc.Address("from")
// 	to := xc.Address("to")
// 	input, err := client.FetchTxInput(s.Ctx, from, to)
// 	require.NotNil(input)
// 	require.EqualError(err, "not implemented")
// }

// func (s *CrosschainTestSuite) TestSubmitTx() {
// 	require := s.Require()
// 	client, _ := NewClient(&xc.ChainConfig{})
// 	err := client.SubmitTx(s.Ctx, &Tx{})
// 	require.EqualError(err, "not implemented")
// }

// func (s *CrosschainTestSuite) TestFetchTxInfo() {
// 	require := s.Require()
// 	client, _ := NewClient(&xc.ChainConfig{})
// 	info, err := client.FetchTxInfo(s.Ctx, xc.TxHash("hash"))
// 	require.NotNil(info)
// 	require.EqualError(err, "not implemented")
// }

// // Tx

// func (s *CrosschainTestSuite) TestTxHash() {
// 	require := s.Require()
// 	tx := Tx{}
// 	require.Equal(xc.TxHash("not implemented"), tx.Hash())
// }

// func (s *CrosschainTestSuite) TestTxSighashes() {
// 	require := s.Require()
// 	tx := Tx{}
// 	sighashes, err := tx.Sighashes()
// 	require.NotNil(sighashes)
// 	require.EqualError(err, "not implemented")
// }

// func (s *CrosschainTestSuite) TestTxAddSignature() {
// 	require := s.Require()
// 	tx := Tx{}
// 	err := tx.AddSignatures([]xc.TxSignature{}...)
// 	require.EqualError(err, "not implemented")
// }
