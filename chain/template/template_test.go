package newchain

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
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

// Address

func (s *CrosschainTestSuite) TestNewAddressBuilder() {
	require := s.Require()
	builder, err := NewAddressBuilder(&xc.ChainConfig{})
	require.NotNil(builder)
	require.EqualError(err, "not implemented")
}

func (s *CrosschainTestSuite) TestGetAddressFromPublicKey() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.ChainConfig{})
	address, err := builder.GetAddressFromPublicKey([]byte{})
	require.Equal(xc.Address(""), address)
	require.EqualError(err, "not implemented")
}

func (s *CrosschainTestSuite) TestGetAllPossibleAddressesFromPublicKey() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.ChainConfig{})
	addresses, err := builder.GetAllPossibleAddressesFromPublicKey([]byte{})
	require.NotNil(addresses)
	require.EqualError(err, "not implemented")
}

// TxBuilder

func (s *CrosschainTestSuite) TestNewTxBuilder() {
	require := s.Require()
	builder, err := NewTxBuilder(&xc.ChainConfig{})
	require.NotNil(builder)
	require.EqualError(err, "not implemented")
}

func (s *CrosschainTestSuite) TestNewNativeTransfer() {
	require := s.Require()
	builder, _ := NewTxBuilder(&xc.ChainConfig{})
	from := xc.Address("from")
	to := xc.Address("to")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	tf, err := builder.(xc.TxTokenBuilder).NewNativeTransfer(from, to, amount, input)
	require.Nil(tf)
	require.EqualError(err, "not implemented")
}

func (s *CrosschainTestSuite) TestNewTokenTransfer() {
	require := s.Require()
	builder, _ := NewTxBuilder(&xc.ChainConfig{})
	from := xc.Address("from")
	to := xc.Address("to")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	tf, err := builder.(xc.TxTokenBuilder).NewTokenTransfer(from, to, amount, input)
	require.Nil(tf)
	require.EqualError(err, "not implemented")
}

func (s *CrosschainTestSuite) TestNewTransfer() {
	require := s.Require()
	builder, _ := NewTxBuilder(&xc.ChainConfig{})
	from := xc.Address("from")
	to := xc.Address("to")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	tf, err := builder.NewTransfer(from, to, amount, input)
	require.Nil(tf)
	require.EqualError(err, "not implemented")
}

// Client

func (s *CrosschainTestSuite) TestNewClient() {
	require := s.Require()
	client, err := NewClient(&xc.ChainConfig{})
	require.NotNil(client)
	require.EqualError(err, "not implemented")
}

func (s *CrosschainTestSuite) TestFetchTxInput() {
	require := s.Require()
	client, _ := NewClient(&xc.ChainConfig{})
	from := xc.Address("from")
	to := xc.Address("to")
	input, err := client.FetchLegacyTxInput(s.Ctx, from, to)
	require.NotNil(input)
	require.EqualError(err, "not implemented")
}

func (s *CrosschainTestSuite) TestSubmitTx() {
	require := s.Require()
	client, _ := NewClient(&xc.ChainConfig{})
	err := client.SubmitTx(s.Ctx, &Tx{})
	require.EqualError(err, "not implemented")
}

func (s *CrosschainTestSuite) TestFetchTxInfo() {
	require := s.Require()
	client, _ := NewClient(&xc.ChainConfig{})
	info, err := client.FetchLegacyTxInfo(s.Ctx, xc.TxHash("hash"))
	require.NotNil(info)
	require.EqualError(err, "not implemented")
}

// Signer

func (s *CrosschainTestSuite) TestNewSigner() {
	require := s.Require()
	signer, err := NewSigner(&xc.ChainConfig{})
	require.NotNil(signer)
	require.EqualError(err, "not implemented")
}

func (s *CrosschainTestSuite) TestImportPrivateKey() {
	require := s.Require()
	signer, _ := NewSigner(&xc.ChainConfig{})
	key, err := signer.ImportPrivateKey("key")
	require.NotNil(key)
	require.EqualError(err, "not implemented")
}

func (s *CrosschainTestSuite) TestSign() {
	require := s.Require()
	signer, _ := NewSigner(&xc.ChainConfig{})
	sig, err := signer.Sign(xc.PrivateKey{}, xc.TxDataToSign{})
	require.NotNil(sig)
	require.EqualError(err, "not implemented")
}

// Tx

func (s *CrosschainTestSuite) TestTxHash() {
	require := s.Require()
	tx := Tx{}
	require.Equal(xc.TxHash("not implemented"), tx.Hash())
}

func (s *CrosschainTestSuite) TestTxSighashes() {
	require := s.Require()
	tx := Tx{}
	sighashes, err := tx.Sighashes()
	require.NotNil(sighashes)
	require.EqualError(err, "not implemented")
}

func (s *CrosschainTestSuite) TestTxAddSignature() {
	require := s.Require()
	tx := Tx{}
	err := tx.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(err, "not implemented")
}

// TxInput

func (s *CrosschainTestSuite) TestSafeFromDoubleSpend() {
	require := s.Require()
	newInput := &TxInput{}
	oldInput1 := &TxInput{}
	// Defaults are false but each chain has conditions
	require.False(newInput.SafeFromDoubleSend(oldInput1))
	require.False(newInput.IndependentOf(oldInput1))
}

func (s *CrosschainTestSuite) TestTxInputConflicts() {
	require := s.Require()
	type testcase struct {
		newInput xc.TxInput
		oldInput xc.TxInput

		independent     bool
		doubleSpendSafe bool
	}
	vectors := []testcase{
		{
			newInput:        &TxInput{},
			oldInput:        &TxInput{},
			independent:     false,
			doubleSpendSafe: false,
		},
		{
			newInput: &TxInput{},
			// check no old input
			oldInput:        nil,
			independent:     false,
			doubleSpendSafe: false,
		},
	}
	for i, v := range vectors {
		newBz, _ := json.Marshal(v.newInput)
		oldBz, _ := json.Marshal(v.oldInput)
		fmt.Printf("testcase %d - expect safe=%t, independent=%t\n     newInput = %s\n     oldInput = %s\n", i, v.doubleSpendSafe, v.independent, string(newBz), string(oldBz))
		fmt.Println()
		require.Equal(
			v.newInput.IndependentOf(v.oldInput),
			v.independent,
			"IndependentOf",
		)
		require.Equal(
			v.newInput.SafeFromDoubleSend(v.oldInput),
			v.doubleSpendSafe,
			"SafeFromDoubleSend",
		)
	}
}
