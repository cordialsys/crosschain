# Sui gRPC client

This package implements `client.Client` and `client.StakingClient` using
`sui.rpc.v2`. The driver factory selects it for Sui. The original JSON-RPC
client remains available as `sui.NewClient`.

```go
import suiclient "github.com/cordialsys/crosschain/chain/sui/client"

cfg := xc.NewChainConfig(xc.SUI).
    WithUrl("https://fullnode.mainnet.sui.io:443").
    WithDecimals(9)
c, err := suiclient.NewClient(cfg)
if err != nil {
    return err
}
defer c.Close()
```

## Configuration

- `url`: gRPC host with optional port. HTTPS or a bare host uses TLS; HTTP
  explicitly selects plaintext. Default ports are 443 and 80 respectively.
  Path prefixes and query strings are preserved for gRPC reverse proxies;
  URL credentials use Basic authentication. The server must support gRPC.
  QuickNode Sui URLs such as `https://name.sui-mainnet.quiknode.pro/TOKEN`
  are translated to the same host on port 9000 with `x-token: TOKEN`, following
  [QuickNode's documentation](https://www.quicknode.com/docs/sui/sui-grpc/overview/go).
  An explicit non-443 port is preserved. The original HTTP URL remains available
  for legacy staking discovery.
- `auth`: optional secret reference sent as Bearer authorization metadata
  (or `x-token` for QuickNode, overriding its URL token).
  Additional provider-specific gRPC options can be passed to `NewClient`.
- `http_timeout`: also bounds each gRPC call. Earlier caller deadlines apply.
- `secondary_url`: optional legacy JSON-RPC URL for `suix_getStakes`. When unset,
  the client uses the HTTP(S) form of `url`. Stake balances and unstaking
  discovery still require a provider serving that legacy method. Coin selection,
  stake object lookup, simulation, and submission use gRPC.

## Builder compatibility

The client returns `sui.TxInput`, `sui.StakingInput` and `sui.UnstakingInput`.
Existing fields and SDK coin JSON encodings are unchanged. Inputs without the
optional `address_balance` field retain the original wire encoding for valid
coin-based transfers. Updated builders also accept the new field; older builders can still
handle coin-only inputs, but cannot spend address balances.

Transfers with Payment Kit metadata and a trusted Transfer ID automatically
combine selected coin objects and address balances. Without both, the builder
uses only coin objects, including for sponsored gas. Staking and unstaking use
only coin objects because they do not execute Payment Kit. The transfer builder
withdraws only the shortfall through `coin::redeem_funds`
inside the same transaction. The optional funding data also supports smashing
multiple gas coins when their individual balances are too small. Native coins
can be smashed for gas; token coins
are merged for spending. Gas comes from the payer's coin objects or SUI address
balance, including sponsored transactions. Sui requires the gas budget to fit
entirely in one of those sources. Input preparation reserves gas before spending
and rejects failed simulations or insufficient funding. Gas estimates reserve
at least the computation charge, even when storage rebates exceed the total
fee. Transfer inputs are checked against the requested asset so an empty token
balance cannot fall back to spending SUI. Coin selection is bounded to 50 per list.

Address-balance gas uses an empty gas-payment list and `ValidDuring` expiration
restricted to the fetched epoch. Chain ID and a random nonce are fetched once
and retained in the input, so offline rebuilds are deterministic. New balance
amounts are decimal strings in JSON. The nonce distinguishes transactions; it
is not an exclusive account sequence. Retry safety requires Payment Kit
deduplication or an exact owned-object reference consumed by both attempts.
Expiration alone cannot prove that an earlier attempt never executed.
Resubmitting the exact same signed bytes remains safe.

Balance queries include both coin-object and address balances. Staking discovery
still uses the legacy `suix_getStakes` method as described above.

Transaction and checkpoint lookups use the configured full node's retention
window. This client does not automatically fall back to an archival service.

## Generated bindings

`rpc/*.pb.go` is generated from the official
[MystenLabs/sui-apis](https://github.com/MystenLabs/sui-apis) definitions at commit
`aa9ba2c13b2a9348059b38110e64af39849a0377`, under Apache-2.0 (`rpc/LICENSE`).
No server reflection is needed at runtime.

To regenerate, check out that commit, install `protoc`, `protoc-gen-go v1.36.11`
and `protoc-gen-go-grpc v1.6.1`, then run:

```sh
python3 chain/sui/client/rpc/generate.py /path/to/sui-apis
```

Run the client and legacy builder tests with `go test -mod=readonly ./chain/sui/...`.

The [2026-10-22 devnet review report](testdata/devnet-review-2026-10-22.json)
records 33 successful transfer cases, 12 expected rejections, and four identical
signed resubmissions without duplicate debits. Coverage includes native and
token transfers, both balance sources, both sponsor types, full-balance sends,
one-unit sends, fragmented gas coins, the 50-coin selection limit, stale inputs,
and missing sponsor signatures. Temporary account balances were returned to
the original account and reconciled against actual gas costs.


## Transfer IDs and Payment Kit

`builder.OptionTransactionID(id)` and the optional
`xc.TxInputWithTransactionID.SetTransactionID(id)` enable
[Mysten's Payment Kit](https://github.com/MystenLabs/sui-payment-kit) for transfers.
An ID identifies **one immutable transfer**, including sender, recipient, amount,
and coin type. Reuse it when rebuilding that transfer; assign a new ID to each
new payment. Arbitrary strings (including Unicode) are hashed with the sender
address into a fixed 32-character nonce. Other applications must use the same
mapping if they want to share this deduplication namespace.

```go
args, err := builder.NewTransferArgs(cfg.Base(), from, to, amount,
    builder.OptionPublicKey(publicKey),
    builder.OptionTransactionID(transferID))
if err != nil { return err }
input, err := c.FetchTransferInput(ctx, args)
if err != nil { return err }
if setter, ok := input.(xc.TxInputWithTransactionID); ok {
    setter.SetTransactionID(transferID) // trusted application data, before conflict checks
}
// Check input.IndependentOf(previousInput), then build and sign normally.
```

The client fetches deployment metadata and estimates the registry write with a
throwaway simulation ID. It leaves `TransactionID` empty. On supported networks,
applications may also fetch without the option and apply just the input setter.
The builder accepts either trusted source and rejects mismatching IDs. Without
an ID, builders retain the ordinary coin-only transfer path and exclude address
funds. No-ID fetches retain coin references and, when both paths are available,
reserve the larger gas estimate; supply the ID option when fetching for the most
precise funding selection. All old `TxInput` fields remain compatible; old
software cannot enforce Payment Kit IDs.

Mainnet and testnet use the
[upstream deployments](https://github.com/MystenLabs/ts-sdks/blob/main/packages/payment-kit/src/constants.ts),
selected by the node's genesis digest. The default registry is derived from the
published namespace; it requires no per-address setup. Devnet/custom networks
require a deployed package and shared registry for protected/address-funded
transfers. Ordinary coin-only transfers remain available without a deployment:

```yaml
sui_payment_kit:
  package_id: "0x..."
  registry_id: "0x..."
```

The client validates the shared registry, version, and supported configuration.
It requires a registry that sends payment directly to the recipient, and reads
its current retention setting. Devnet resets can invalidate custom deployments.
The offline builder also checks the package and registry against its trusted
configuration or the built-in mainnet/testnet deployment. Configure custom
deployments on both the client and builder; metadata in `TxInput` cannot select
arbitrary Move code for the signer to execute.
The CLI accepts the same overrides with `--sui-payment-kit-package` and
`--sui-payment-kit-registry`.

`IndependentOf` rejects the same logical ID, even across epochs. Different IDs
are independent when their consumed owned-object references do not overlap (or
the older input expired). Protected transfers exclude unused sponsor gas coins
from queuing; legacy inputs retain conservative checks. Retry safety only counts
objects guaranteed to be consumed. Stake objects are
included for unstaking. An expired input can release a queue conflict, but cannot
by itself authorize retrying a payment that might already have executed.
When an ID is supplied during fetching and address funds cover the amount and gas, the client selects
those funds to avoid owned-coin dependencies. It re-estimates gas after changing
funding sources, retaining the coin plan if address funds cannot cover the new
fee. Mixed/coin-only funding still
requires current object versions; shared registry access is ordered by Sui and
does not require fetching a new registry version after every payment.

`SafeFromDoubleSend` uses the ID, sender, deployment, coin type, and record
retention window. Payment Kit's key also includes amount and recipient, so
**never change those while reusing an ID**. Its default retention is 30 epochs;
records can be deleted afterward. Crosschain rejects retry safety once that
window is reached, even if the ID is unchanged. This assumes a trusted registry
administrator, who can change retention. These are bounded deduplication
semantics, not a permanent account nonce.

## CLI transfer history

`xc transfer` creates a random UUID by default, or accepts a stable `--id`:

```sh
xc --chain SUI --rpc https://fullnode.testnet.sui.io:443 transfer ADDRESS 0.01 --id 42
```

The CLI caches transfer intent and `TxInput` under the user's cache directory
(`xc/transfers`), grouped by chain/network/RPC and sender. Use `--history-dir` or
`XC_TRANSFER_HISTORY_DIR` to select a directory. This feature works for all
chains: it checks each previous input using the driver's `IndependentOf` and
also rejects reused IDs. Each check prints one concise line. On conflict it
waits three seconds, fetches fresh input, reapplies trusted options, and checks
again, for up to 30 minutes (`--conflict-timeout` can shorten this).

Concurrent local processes reserve an input under a directory lock before
signing, and save the final input and hash before broadcast. Ambiguous broadcast
failures keep the record; pre-broadcast build failures remove the reservation.
Dry runs do not check or write history. `--input` conflicts fail because that
option promises not to fetch from RPC. History is a best-effort debugging aid:
unreadable records are reported as skipped, cache failures warn and proceed,
and switching RPC URLs or deleting the cache loses local deduplication. A stale
`.lock` left by a crashed process can be removed after that process is stopped.

The [2026-10-23 Payment Kit testnet report](testdata/payment-kit-testnet-2026-10-23.json)
includes transaction hashes, gas costs, cached inputs, and live queuing logs.
It records seven distinct IDs in one checkpoint, an on-chain duplicate rejection,
parallel CLI processes, and a coin conflict resolved by refetching. All temporary
account funds were recovered. It also records the failed probes that exposed the
funding-change estimation bug, which is covered by regression tests.

The [2026-10-23 cleanup stress report](testdata/cleanup-stress-testnet-2026-10-23.json)
records 59 successful native SUI transactions, four expected on-chain duplicate
rejections, and five identical signed resubmissions without additional debits.
Twenty-four distinct IDs succeeded in one checkpoint. Coverage includes mixed
funding, sponsored gas, fragmented coins, legacy transfers, and concurrent CLI
history checks. All temporary balances were recovered; fees reconcile exactly
after accounting for a separate incoming 1 SUI deposit. Tokens and staking were
covered by automated tests, not live transactions in that run.
