# Address Normalization

Cordial Systems APIs use blockchain addresses as identifiers for both addresses and assets, for example

- `chains/ETH/addresses/0x4e27f5efe252fb7fbee02f503be881ac2d20f679` is the name of an address on Ethereum, and
- `chains/ETH/assets/0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48` is the name of an asset on Ethereum (the USDC stablecoin)

Two steps of normalization are in play:

- non-destructive / canonicalization: Some chains use multiple equivalent formats for an address, for instance Ethereum uses 0x-prefixed lower-case hex, but also a mixed-case variant encoding a checksum
- destructive / irreversible: As a general principle, in Cordial APIs one can "get" a resource by sending an HTTP GET request corresponding to the resource name. Hence we restrict names to contain characters that are valid in a URL, and more specifically, to alphanumeric (`[0-9A-Za-z]`), dash (`-`) and underscore (`_`).

The first step is chain-specific, `crosschain` defines and uses a single canonical format for each address on each chain. The intent of this document is to specify this canonical format, for all chains supported by `crosschain`.

The second step is generic, we replace whitespace with underscore (`_`) and all other invalid characters with dashes (`-`). Since this is not reversible, an address resource will have a field `address` containing the original (canonical) address, and a (token) asset resource will have a field `contract` containing the original (canonical) address. Note that this second step is not used in `crosschain` itself, although there is an implementation in the function `normalizeId` in [`normalize/id_test.go`](../normalize/id_test.go) for compatibility testing purposes.

## Specification

The function `Normalize` in [`normalize/normalize.go`](../normalize/normalize.go) implements the (non-destructive) conversion of an address to its canonical format. In the following, we specify this implementation.

Note that https://connector.cordialapis.com/v1/chains lists all currently supported chains, together with their symbol.

**BTC/BCH**: For Bitcoin Cash, a possible `bitcoincash:` prefix is removed

**ETH** and other EVM chains: The mixed-case checksum encoding is removed, addresses are lower-case and include the `0x` prefix

**APTOS/SUI**: Addresses are lower-case with `0x` prefix. For assets, there is a suffix, such as `0x1::aptos_coin::AptosCoin` or `0xf22bede237a07e121b56d91a491eb7bcdfd1f5907926a9e58338f964a01b17fa::asset::USDC`. The leading hexadecimal part is lower-cased, the two following segments kept as-is. Note that this is a case where the second normalization is non-trivial, the Cordial APIs name of the `APT` asset is `chains/APTOS/assets/0x1--aptos_coin--AptosCoin`.

**ATOM** and other Cosmos chains: Nothing to do, bech32 format is used.

**SOL**: Nothing to do, base58 format is used

**TRON**: Multiple formats are in use, we use base64

