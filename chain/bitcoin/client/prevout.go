package client

import (
	"bytes"
	"context"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
)

// SupportsLegacyPrevoutVerification lists chains whose standard transactions
// can be authenticated using Bitcoin's serialization and txid algorithm.
func SupportsLegacyPrevoutVerification(chain xc.NativeAsset) bool {
	_, ok := legacyMaxMoney(chain)
	return ok
}

func legacyMaxMoney(chain xc.NativeAsset) (int64, bool) {
	// Consensus MAX_MONEY bounds, in each chain's smallest unit.
	switch chain {
	case xc.BTC:
		return 21_000_000 * 100_000_000, true
	case xc.LTC:
		// https://github.com/litecoin-project/litecoin/blob/master/src/amount.h
		return 84_000_000 * 100_000_000, true
	case xc.DOGE:
		// https://github.com/dogecoin/dogecoin/blob/master/src/amount.h
		return 10_000_000_000 * 100_000_000, true
	default:
		return 0, false
	}
}

// FetchLegacyPrevouts verifies selected legacy UTXOs against their parent txids,
// reported amounts and sender scripts. sourceScript must derive the script from
// the locally supplied sender address, not from the connector's response.
// Parents are fetched, decoded and hashed once per call and are not retained in
// the returned inputs. The builder trusts these inputs and does not reverify them.
// This does not establish confirmation or whether an output is still unspent.
// Parents must use standard Bitcoin serialization, optionally with witness;
// extension formats such as Litecoin MWEB are not supported by this decoder.
func FetchLegacyPrevouts(ctx context.Context, chain xc.NativeAsset, outputs []tx_input.Output, sourceScript func(xc.Address) ([]byte, error), fetch func(context.Context, string) ([]byte, error)) error {
	maxMoney, supported := legacyMaxMoney(chain)
	if !supported {
		return nil
	}
	parents := make(map[chainhash.Hash]*wire.MsgTx)
	scripts := make(map[xc.Address][]byte)
	for i := range outputs {
		output := &outputs[i]
		expectedScript, ok := scripts[output.Address]
		if !ok {
			var err error
			expectedScript, err = sourceScript(output.Address)
			if err != nil {
				return err
			}
			scripts[output.Address] = expectedScript
		}
		// Use the trusted sender to choose the sighash path. A connector cannot
		// skip legacy verification by supplying a witness script for a UTXO.
		if chain != xc.DOGE && (txscript.IsPayToWitnessPubKeyHash(expectedScript) || txscript.IsPayToTaproot(expectedScript)) {
			output.PubKeyScript = bytes.Clone(expectedScript)
			continue
		}
		hash, err := chainhash.NewHash(output.Hash)
		if err != nil {
			return fmt.Errorf("invalid legacy outpoint hash: %w", err)
		}
		parent, ok := parents[*hash]
		if !ok {
			raw, err := fetch(ctx, hash.String())
			if err != nil {
				return fmt.Errorf("could not fetch previous transaction %s: %w", hash, err)
			}
			if len(raw) == 0 {
				return fmt.Errorf("missing previous transaction %s", hash)
			}
			parent = new(wire.MsgTx)
			reader := bytes.NewReader(raw)
			if err := parent.Deserialize(reader); err != nil {
				return fmt.Errorf("invalid previous transaction: %w", err)
			}
			if reader.Len() != 0 {
				return fmt.Errorf("trailing data in previous transaction")
			}
			// TxHash excludes witness data, as required for an outpoint's txid.
			if parent.TxHash() != *hash {
				return fmt.Errorf("previous transaction txid does not match outpoint %s", output.Outpoint.String())
			}
			parents[*hash] = parent
		}
		if uint64(output.Index) >= uint64(len(parent.TxOut)) {
			return fmt.Errorf("previous transaction output index %d is out of range", output.Index)
		}
		prevout := parent.TxOut[output.Index]
		if prevout.Value < 0 || prevout.Value > maxMoney {
			return fmt.Errorf("invalid previous transaction output value")
		}
		value := xc.NewAmountBlockchainFromUint64(uint64(prevout.Value))
		if output.Value.Cmp(&value) != 0 {
			return fmt.Errorf("previous transaction output value mismatch for %s", output.Outpoint.String())
		}
		if !matchesSourceScript(chain, prevout.PkScript, expectedScript) {
			return fmt.Errorf("input script does not match sender %s", output.Address)
		}
		// Blockchair supplies an address-level script, which can differ from
		// an individual output's script. Keep only the authenticated script;
		// copying it avoids retaining the parent's shared script buffer.
		output.PubKeyScript = bytes.Clone(prevout.PkScript)
	}
	return nil
}

func matchesSourceScript(chain xc.NativeAsset, script, expectedScript []byte) bool {
	if bytes.Equal(expectedScript, script) {
		return true
	}
	if chain == xc.DOGE && txscript.IsPayToPubKey(script) && txscript.IsPayToPubKeyHash(expectedScript) {
		// A standard P2PK script is a single public-key push followed by CHECKSIG.
		publicKey := script[1 : len(script)-1]
		return bytes.Equal(btcutil.Hash160(publicKey), expectedScript[3:23])
	}
	return false
}
