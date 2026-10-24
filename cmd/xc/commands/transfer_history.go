package commands

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// This is a local debugging aid, not an application transaction database. Store
// intent before broadcasting: a timeout does not prove a transfer failed.
type transferHistoryRecord struct {
	ID       string             `json:"id"`
	Created  time.Time          `json:"created"`
	From     xc.Address         `json:"from"`
	To       xc.Address         `json:"to"`
	Amount   string             `json:"amount"`
	Contract xc.ContractAddress `json:"contract,omitempty"`
	Hash     xc.TxHash          `json:"hash,omitempty"`
	Input    json.RawMessage    `json:"input"`
}
type transferHistory struct{ dir string }
type transferReservation struct {
	history *transferHistory
	path    string
	record  transferHistoryRecord
}

func newTransferHistory(root string, chain *xc.ChainConfig, from xc.Address) (*transferHistory, error) {
	if root == "" {
		root = os.Getenv("XC_TRANSFER_HISTORY_DIR")
		if root == "" {
			cache, err := os.UserCacheDir()
			if err != nil {
				return nil, err
			}
			root = filepath.Join(cache, "xc", "transfers")
		}
	}
	// Do not store credentials in a filename. Distinguish network/RPC overrides;
	// switching providers intentionally starts a separate best-effort history.
	key := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", chain.Chain, chain.Network, chain.ChainID, chain.URL, from)
	digest := sha256.Sum256([]byte(key))
	dir := filepath.Join(root, fmt.Sprintf("%x", digest))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &transferHistory{dir: dir}, nil
}

func (h *transferHistory) lock(ctx context.Context) (func(), error) {
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		err := os.Mkdir(filepath.Join(h.dir, ".lock"), 0700)
		if err == nil {
			return func() { _ = os.Remove(filepath.Join(h.dir, ".lock")) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-deadline.C:
			timer.Stop()
			return nil, fmt.Errorf("history lock is busy (remove a stale .lock directory after a crashed process)")
		case <-timer.C:
		}
	}
}

func (h *transferHistory) reserve(ctx context.Context, id string, args builder.TransferArgs, input xc.TxInput) (*transferReservation, bool, error) {
	unlock, err := h.lock(ctx)
	if err != nil {
		return nil, false, err
	}
	defer unlock()
	entries, err := os.ReadDir(h.dir)
	if err != nil {
		return nil, false, err
	}
	checked, conflicts, skipped := 0, 0, 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(h.dir, entry.Name()))
		if err != nil {
			skipped++
			continue
		}
		var previous transferHistoryRecord
		if err = json.Unmarshal(data, &previous); err != nil {
			skipped++
			continue
		}
		if previous.ID == id {
			checked++
			conflicts++
			continue
		}
		old, err := drivers.UnmarshalTxInput(previous.Input)
		if err != nil {
			skipped++
			continue
		}
		checked++
		if !input.IndependentOf(old) {
			conflicts++
		}
	}
	fmt.Fprintf(os.Stderr, "transfer history: id=%q checked=%d conflicts=%d skipped=%d\n", id, checked, conflicts, skipped)
	if conflicts > 0 {
		return nil, true, nil
	}
	raw, err := drivers.MarshalTxInput(input)
	if err != nil {
		return nil, false, err
	}
	contract, _ := args.GetContract()
	r := &transferReservation{history: h, path: filepath.Join(h.dir, uuid.NewString()+".json"), record: transferHistoryRecord{ID: id, Created: time.Now().UTC(), From: args.GetFrom(), To: args.GetTo(), Amount: args.GetAmount().String(), Contract: contract, Input: raw}}
	return r, false, r.write()
}
func (r *transferReservation) write() error {
	data, err := json.MarshalIndent(r.record, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(r.history.dir, ".transfer-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), r.path)
}
func (r *transferReservation) update(ctx context.Context, args builder.TransferArgs, input xc.TxInput, hash xc.TxHash) error {
	if r == nil {
		return nil
	}
	unlock, err := r.history.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	raw, err := drivers.MarshalTxInput(input)
	if err != nil {
		return err
	}
	r.record.Input = raw
	r.record.Hash = hash
	r.record.Amount = args.GetAmount().String()
	return r.write()
}
func (r *transferReservation) discard() {
	if r != nil {
		_ = os.Remove(r.path)
	}
}

func waitForTransferInput(ctx context.Context, h *transferHistory, id string, args builder.TransferArgs, initial xc.TxInput, fetch func(context.Context) (xc.TxInput, error), prepare func(xc.TxInput) (xc.TxInput, error), timeout, interval time.Duration) (xc.TxInput, *transferReservation, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	input := initial
	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		var err error
		input, err = prepare(input)
		if err != nil {
			return nil, nil, err
		}
		r, conflict, err := h.reserve(ctx, id, args, input)
		if err != nil {
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			logrus.WithError(err).Warn("transfer history unavailable; continuing without local conflict protection")
			return input, nil, nil
		}
		if !conflict {
			return input, r, nil
		}
		if fetch == nil {
			return nil, nil, fmt.Errorf("transfer input conflicts with history; --input cannot refresh from RPC")
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, nil, fmt.Errorf("transfer conflicts persisted: %w", ctx.Err())
		case <-timer.C:
		}
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		input, err = fetch(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("refresh transfer input: %w", err)
		}
	}
}
