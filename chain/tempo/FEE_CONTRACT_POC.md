# Tempo fee-contract + fee-payer PoC

`builder.OptionFeeContract` selects the TIP-20 token used for transaction fees,
independently of `OptionContractAddress`, which selects the token being sent.
Combine it with the existing `OptionFeePayer` to debit a separate signing
address's balance in that token. Both options apply to the same transaction.

```go
args, err := builder.NewTransferArgs(
    chain.Base(), from, to, amount,
    builder.OptionContractAddress(tokenToSend),
    builder.OptionFeeContract(tokenForFees),
    builder.OptionFeePayer(payerAddress, payerPublicKey),
)
if err != nil { return err }

// Use the Tempo client and builder, or obtain them through the factory.
input, err := client.FetchTransferInput(ctx, args)
if err != nil { return err }
transaction, err := txBuilder.Transfer(args, input)
if err != nil { return err }
requests, err := transaction.Sighashes()
if err != nil { return err }
signatures, err := senderSigner.SignAll(requests)
if err != nil { return err }
if err := transaction.SetSignatures(signatures...); err != nil { return err }
requests, err = transaction.(xc.TxAdditionalSighashes).AdditionalSighashes()
if err != nil { return err }
payerSignatures, err := payerSigner.SignAll(requests)
if err != nil { return err }
signatures = append(signatures, payerSignatures...)
if err := transaction.SetSignatures(signatures...); err != nil { return err }
raw, err := transaction.Serialize() // 0x76 || RLP(fields..., signature)
if err != nil { return err }
// Existing SubmitTx / SubmitTxReqFromTx supports these bytes unchanged.
_ = raw
```

The option activates `FeeTokenTx`, a small `xc.Tx` implementation in the Tempo
package. It reuses EVM ERC-20 transfer calldata, nonce/gas-price fetching, the
existing K256 signers, two-round signature interface, and raw transaction
submission. The input records the chosen fee contract and payer, so
`GetFeeLimit` returns the fee in that token's six-decimal units. The builder
rejects a mismatch between the input's fee currency and the requested currency.

The serializer does not fit the internal EVM `evmTx` interface: that interface
requires `BuildEthTx() *types.Transaction`, while this geth version does not
support Tempo's custom type. Implementing `xc.Tx` directly keeps the change
inside the existing Tempo driver without a geth fork or changes to EVM hashing.

The combined path uses Tempo's native sponsorship with two signatures:

1. `Sighashes()` requests the sender signature over `keccak256(0x76 || RLP(...))`.
   This preimage clears `feeToken` and sets the sponsorship marker to `0x00`.
2. `AdditionalSighashes()` requests the selected payer's signature over
   `keccak256(0x78 || RLP(...))`, including the fee token and sender address.
3. `SetSignatures(sender, payer)` verifies both recovered signing addresses.
   The final `0x76` transaction includes `feeToken`, the payer signature as
   `[yParity, r, s]`, and the sender signature as `r || s || v` with v=27/28.

The payer address is recovered from its signature on chain. The sender does not
cryptographically choose the sponsor or fee currency; the sponsor commits to
the currency. Locally, the builder requires the requested payer and fee currency
to match the input, and checks signatures against the requested addresses.

Only the sender's normal sequential nonce (`nonceKey = 0`) is consumed. There is
no EIP-7702 delegation, smart-account nonce, or fee-payer nonce in this path.
`NativeFeePayer` records this distinction in serialized transaction inputs and
prevents a shared sponsor from introducing false nonce conflicts between senders.

For sponsored transfers, the PoC uses `GasLimitDefault` (500,000 if unset).
It marks that gas budget as unestimated via `IsFeeLimitAccurate() == false`.
Faithful sponsored estimation needs additional RPC work; it must not silently
estimate a sender-paid transaction when the sender has no fee-token balance.
The fee limit still reports the maximum budget in the chosen token's units.

This PoC covers single token transfers. Batching, CLI flags, access keys,
passkeys, and scheduling are outside its scope. Without `OptionFeePayer`, the
explicit fee-token path supports sender-paid fees. Without `OptionFeeContract`,
the existing EVM transaction path remains in use.

Fee tokens must be eligible USD-denominated TIP-20 contracts with sufficient
payer balance and fee-AMM liquidity. The node checks eligibility and execution.
Tests compare both signing payloads and the complete sponsored transaction with
an Ox-generated vector, reject wrong signers, verify sponsor commitment to the
fee token, and cover input fetching and nonce handling. No live funded broadcast
has been performed.

Run:

```sh
go test -mod=readonly ./chain/tempo ./chain/evm/... ./builder/...
```

`testdata/fee_token_vector.json` is generated with Ox 0.14.0 using the adjacent
script and a public test key. Normal Go tests require no JavaScript dependency.

References: [Tempo fee rules](https://docs.tempo.xyz/protocol/fees/spec-fee),
[reference transaction encoding](https://github.com/tempoxyz/tempo/blob/main/crates/primitives/src/transaction/tempo_transaction.rs),
[signature encoding](https://github.com/tempoxyz/tempo/blob/main/crates/primitives/src/transaction/tt_signature.rs),
[Ox serializer](https://github.com/wevm/ox/blob/main/src/tempo/TxEnvelopeTempo.ts).
