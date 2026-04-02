This directory contains generated Go protobuf/grpc types used by the Canton integration.

From [`chain/canton`](/Users/conorpatrick/crosschain/chain/canton), regenerate them from checked-in proto source with:

```
just pb
```

Proto sources live in:

- [`chain/canton/proto`](/Users/conorpatrick/crosschain/chain/canton/proto)

Upstream source provenance:

- `digital-asset/canton`
- `community/ledger-api-proto/src/main/protobuf/com/daml/ledger/api/v2`
- `community/daml-lf/ledger-api-value/src/main/protobuf/com/daml/ledger/api/v2/value.proto`
