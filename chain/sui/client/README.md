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
optional `address_balance` field retain the original builder behavior and wire
encoding. Updated builders also accept the new field; older builders can still
handle coin-only inputs, but cannot spend address balances.

Transfers and staking automatically combine selected coin objects and address
balances. The builder withdraws only the shortfall through `coin::redeem_funds`
inside the same transaction. Native coins can be smashed for gas; token coins
are merged for spending. Gas comes from the payer's coin objects or SUI address
balance, including sponsored transactions. Sui requires the gas budget to fit
entirely in one of those sources. Input preparation reserves gas before spending
and rejects insufficient funding. Coin selection is bounded to 50 per list.

Address-balance gas uses an empty gas-payment list and `ValidDuring` expiration
restricted to the fetched epoch. Chain ID and a random nonce are fetched once
and retained in the input, so offline rebuilds are deterministic. New balance
amounts are decimal strings in JSON. The nonce distinguishes transactions; it
is not an exclusive account sequence. Retry-safety checks conservatively reject
re-signing with address-balance inputs in the same epoch. Resubmitting the exact
same signed bytes remains safe. After expiration, fresh inputs can be fetched.

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
