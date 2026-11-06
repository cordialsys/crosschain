# Zcash transactions

ZEC transfers use the transparent-only v5 format from
[ZIP 225](https://zips.z.cash/zip-0225), with transaction identifiers and
`SIGHASH_ALL` signature digests from [ZIP 244](https://zips.z.cash/zip-0244).
The builder spends P2PKH outputs and can pay P2PKH or P2SH destinations.
Shielded transfers and spending P2SH outputs are not implemented.

The default consensus branch is NU7 (`0x77190AD9`) on both mainnet and testnet.
The Zcash `chain_id` configuration field overrides the consensus branch for
custom networks; it must be a nonzero unsigned 32-bit integer. Branch selection
is configuration based, not discovered from the RPC node. `FetchTransferInput`
and `FetchMultiTransferInput` preserve that branch in `consensus_branch_id` for
offline construction and signing. Manually constructed inputs must supply it,
along with each previous output's 32-byte transaction hash (wire byte order),
amount, and P2PKH script.

The shared Flux driver retains v4 serialization and ZIP 243 signing. Zcash's
network upgrade is not applied to Flux. Fee estimates and confirmation policy
are unchanged by this upgrade.

Deploy with a node and indexer that support NU7. The current JSON-RPC backend
uses Blockbook REST for UTXO discovery, so a standalone Zebra RPC endpoint does
not replace the indexer. Configure `indexer_url` when it differs from `url`.

## Verification

Run `go test -mod=readonly -tags not_ci ./chain/zcash/... ./chain/bitcoin/...`.
The tests cover v5 serialization, transaction IDs, signature digests, signing,
invalid inputs/signatures, and preservation of Flux's legacy encoding.

`testdata/transparent_v5.json` was generated using the unmodified serialization
and hashing functions in the Zcash reference implementation, pinned to commit
`78321beacb0e0477e33cd002b56585a107c2708c`. Its published ZIP 244 vectors include
shielded bundles, so `testdata/generate_v5.py` constructs transparent fixtures
for those reference functions. To reproduce them with a checkout at that commit:

```sh
python3 chain/zcash/testdata/generate_v5.py /path/to/zcash-test-vectors > /tmp/transparent_v5.json
diff chain/zcash/testdata/transparent_v5.json /tmp/transparent_v5.json
```
