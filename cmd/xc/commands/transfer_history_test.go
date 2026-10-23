package commands

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	evm "github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/cordialsys/crosschain/chain/sui"
	"github.com/stretchr/testify/require"
)

func historyArgs(t *testing.T) (*xc.ChainConfig, builder.TransferArgs) {
	cfg := xc.NewChainConfig(xc.ETH).WithUrl("http://localhost:1234")
	args, err := builder.NewTransferArgs(cfg.Base(), "sender", "receiver", xc.NewAmountBlockchainFromUint64(1))
	require.NoError(t, err)
	return cfg, args
}
func evmInput(n uint64) xc.TxInput { in := evm.NewTxInput(); in.Nonce = n; return in }
func TestTransferHistoryRefreshAndPersist(t *testing.T) {
	cfg, args := historyArgs(t)
	h, err := newTransferHistory(t.TempDir(), cfg, args.GetFrom())
	require.NoError(t, err)
	old, conflict, err := h.reserve(context.Background(), "first", args, evmInput(4))
	require.NoError(t, err)
	require.False(t, conflict)
	require.NoError(t, old.update(context.Background(), args, evmInput(4), "submitted-hash"))
	fetches, prepared := 0, 0
	prepare := func(in xc.TxInput) (xc.TxInput, error) { prepared++; return in, nil }
	in, r, err := waitForTransferInput(context.Background(), h, "next", args, evmInput(4), func(context.Context) (xc.TxInput, error) { fetches++; return evmInput(5), nil }, prepare, time.Second, time.Millisecond)
	require.NoError(t, err)
	require.NotNil(t, r)
	require.EqualValues(t, 5, in.(*evm.TxInput).Nonce)
	require.Equal(t, 1, fetches)
	require.Equal(t, 2, prepared)
	raw, err := os.ReadFile(old.path)
	require.NoError(t, err)
	var record transferHistoryRecord
	require.NoError(t, json.Unmarshal(raw, &record))
	require.Equal(t, xc.TxHash("submitted-hash"), record.Hash)
	require.Contains(t, string(record.Input), `"nonce": 4`)
	r.discard()
	_, err = os.Stat(r.path)
	require.True(t, os.IsNotExist(err))
}
func TestTransferHistoryDuplicateTimeoutAndOffline(t *testing.T) {
	cfg, args := historyArgs(t)
	h, err := newTransferHistory(t.TempDir(), cfg, args.GetFrom())
	require.NoError(t, err)
	_, _, err = h.reserve(context.Background(), "42", args, evmInput(1))
	require.NoError(t, err)
	prepare := func(in xc.TxInput) (xc.TxInput, error) { return in, nil }
	_, _, err = waitForTransferInput(context.Background(), h, "42", args, evmInput(2), func(context.Context) (xc.TxInput, error) { return evmInput(3), nil }, prepare, 15*time.Millisecond, time.Millisecond)
	require.ErrorContains(t, err, "deadline exceeded")
	_, _, err = waitForTransferInput(context.Background(), h, "42", args, evmInput(2), nil, prepare, time.Second, time.Millisecond)
	require.ErrorContains(t, err, "--input")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = waitForTransferInput(ctx, h, "42", args, evmInput(2), func(context.Context) (xc.TxInput, error) { t.Fatal("fetch after cancellation"); return nil, nil }, prepare, time.Second, time.Millisecond)
	require.Error(t, err)
}
func TestTransferHistoryAtomicReservation(t *testing.T) {
	cfg, args := historyArgs(t)
	h, err := newTransferHistory(t.TempDir(), cfg, args.GetFrom())
	require.NoError(t, err)
	var wg sync.WaitGroup
	outcomes := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, conflict, err := h.reserve(context.Background(), "same", args, evmInput(1))
			if err != nil {
				t.Error(err)
			}
			outcomes <- conflict
		}()
	}
	wg.Wait()
	close(outcomes)
	successes := 0
	for conflict := range outcomes {
		if !conflict {
			successes++
		}
	}
	require.Equal(t, 1, successes)
}
func TestTransferHistorySuiIDsAndBestEffort(t *testing.T) {
	cfg, args := historyArgs(t)
	h, err := newTransferHistory(t.TempDir(), cfg, args.GetFrom())
	require.NoError(t, err)
	makeInput := func(id string) *sui.TxInput {
		in := sui.NewTxInput()
		in.PaymentKit = &sui.PaymentKitInput{ChainID: "testnet"}
		in.SetTransactionID(id)
		return in
	}
	_, _, err = h.reserve(context.Background(), "42", args, makeInput("42"))
	require.NoError(t, err)
	_, conflict, err := h.reserve(context.Background(), "43", args, makeInput("43"))
	require.NoError(t, err)
	require.False(t, conflict)
	require.NoError(t, os.WriteFile(filepath.Join(h.dir, "corrupt.json"), []byte("corrupt"), 0600))
	in, r, err := waitForTransferInput(context.Background(), h, "44", args, makeInput("44"), nil, func(in xc.TxInput) (xc.TxInput, error) { return in, nil }, time.Second, time.Millisecond)
	require.NoError(t, err)
	require.NotNil(t, in)
	require.NotNil(t, r)
	other, err := newTransferHistory(filepath.Dir(h.dir), cfg, "another-address")
	require.NoError(t, err)
	require.NotEqual(t, h.dir, other.dir)
}

func TestTransferHistoryKeepsKnownIDConflictWithUnreadableInput(t *testing.T) {
	cfg, args := historyArgs(t)
	h, err := newTransferHistory(t.TempDir(), cfg, args.GetFrom())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(h.dir, "old.json"), []byte(`{"id":"42","input":{}}`), 0600))
	r, conflict, err := h.reserve(context.Background(), "42", args, evmInput(100))
	require.NoError(t, err)
	require.True(t, conflict)
	require.Nil(t, r)
}
