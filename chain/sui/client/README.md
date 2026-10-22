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
  Paths, query strings and URL credentials are rejected; use your provider's
  native gRPC endpoint.
- `auth`: optional secret reference sent as Bearer authorization metadata.
  Additional provider-specific gRPC options can be passed to `NewClient`.
- `http_timeout`: also bounds each gRPC call. Earlier caller deadlines apply.
- `secondary_url`: optional legacy JSON-RPC URL for `suix_getStakes`. When unset,
  the client uses the HTTP(S) form of `url`. Stake balances and unstaking
  discovery still require a provider serving that legacy method. Coin selection,
  stake object lookup, simulation, and submission use gRPC.

## Builder compatibility

The client returns the existing `sui.TxInput`, `sui.StakingInput` and
`sui.UnstakingInput`, including the existing SDK coin types and JSON encoding.
The existing offline builders, BCS transaction format and signature envelopes
are unchanged. Native/token transfers and sponsored transactions continue to
use coin objects, including a coin object for gas payment.

Balance queries report the total of coin-object and address balances. Spending
address balances is not supported by these existing builders; this client does
not synthesize coin references or introduce new input variants to spend them.

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
