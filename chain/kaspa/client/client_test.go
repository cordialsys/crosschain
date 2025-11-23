package client_test

import (
	"context"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/template/client"
	"github.com/cordialsys/crosschain/chain/template/tx"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {

	client, err := client.NewClient(xc.NewChainConfig(""))
	require.NotNil(t, client)
	require.EqualError(t, err, "not implemented")
}

func TestFetchTxInput(t *testing.T) {

	client, _ := client.NewClient(xc.NewChainConfig(""))
	from := xc.Address("from")
	to := xc.Address("to")
	input, err := client.FetchLegacyTxInput(context.Background(), from, to)
	require.NotNil(t, input)
	require.EqualError(t, err, "not implemented")
}

func TestSubmitTx(t *testing.T) {

	client, _ := client.NewClient(xc.NewChainConfig(""))
	tx, err := xctypes.SubmitTxReqFromTx(xc.KAS, &tx.Tx{})
	require.EqualError(t, err, "failed to serialize tx: not implemented")
	err = client.SubmitTx(context.Background(), tx)
	require.EqualError(t, err, "not implemented")
}

func TestFetchTxInfo(t *testing.T) {

	client, _ := client.NewClient(xc.NewChainConfig(""))
	info, err := client.FetchLegacyTxInfo(context.Background(), xc.TxHash("hash"))
	require.NotNil(t, info)
	require.EqualError(t, err, "not implemented")
}
