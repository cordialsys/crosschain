# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Crosschain is a Go library for interacting with multiple blockchains. The main design principle is to isolate functionality into separate **Client**, **Signer**, and **TxBuilder** interfaces, providing unified interfaces for all supported chains while allowing blockchains to be safely used in secure contexts.

## Key Commands

### Build and Test

```bash
# Build all packages
make all
# or with CGO disabled (default in Makefile)
CGO_ENABLED=0 go build -v ./...

# Run tests (excludes CI integration tests)
make test
# or directly
go test -mod=readonly -tags not_ci ./...

# Lint code
make lint

# Format code
make fmt

# Install the xc CLI utility
make install
# or directly
CGO_ENABLED=0 go install -v ./cmd/...
```

### Testing Specific Chains

```bash
# Run tests for a specific chain
go test ./chain/bitcoin/...
go test ./chain/evm/...
go test ./chain/solana/...

# Run a single test
go test -run TestName ./path/to/package
```

### Using the xc CLI

The `xc` utility demonstrates library usage and is useful for manual testing:

```bash
# Install
go install -v ./cmd/xc/...

# Set up environment (required for most commands)
# For mainnet:
source .envrc-testnet
# Or, for testnet:
source .envrc-mainnet

# Common commands
xc address --chain SOL
xc balance <address> --chain ETH
xc transfer <destination> <amount> -v --chain SOL
xc tx-info --chain BTC <tx-hash>
xc tx-input <from-address> --chain ETH
xc staking stake --amount 0.1 --chain SOL --validator <validator-address>

# Use custom RPC
xc balance <address> --chain ETH --rpc https://your-rpc-url
```

## Architecture

### Three-Interface Design

The library separates concerns into three main interfaces:

1. **Client** (`client/client.go`) - Fetches data from blockchain and submits transactions

   - `FetchTransferInput()` - Get transaction inputs (gas, nonces, UTXOs)
   - `FetchBalance()` - Query balances
   - `FetchTxInfo()` - Get transaction details
   - `SubmitTx()` - Broadcast signed transactions
   - `FetchBlock()` - Retrieve block data
   - Extensions: `StakingClient`, `MultiTransferClient`

2. **TxBuilder** (`builder/builder.go`) - Constructs unsigned transactions

   - `Transfer()` - Build transfer transactions
   - `MultiTransfer()` - Build multi-destination transfers
   - `Stake()`, `Unstake()`, `Withdraw()` - Build staking transactions

3. **Signer** (`factory/signer/signer.go`) - Signs transaction payloads
   - Supports multiple signature algorithms: Ed25519, K256 (Keccak/SHA256), Schnorr, BLS12-381
   - Environment variable `XC_SIGN_WITH_SCALAR=1` enables scalar signing for Ed25519 (for MPC compatibility)

### Chain Implementation Structure

Each chain implementation follows a consistent pattern in `chain/<chain-name>/`:

- `client/client.go` - Chain-specific client implementation
- `builder/builder.go` - Transaction builder for the chain
- `address/` - Address derivation and validation
- `tx/` or `tx_input/` - Transaction and input structures
- `signer.go` (some chains) - Chain-specific signing logic

### Factory Pattern

The `factory/` package provides a factory pattern for constructing chain-specific components:

- `factory/main.go` - Main factory implementation
- `factory/drivers/` - Driver registry for all chains
- `factory/defaults/chains/` - Default chain configurations (mainnet.yaml, testnet.yaml)
- `factory/new.go` - Factory initialization

Use the factory to create clients, builders, and signers:

```go
xcFactory := factory.NewDefaultFactory()
client, err := xcFactory.NewClient(chainConfig)
txBuilder, err := xcFactory.NewTxBuilder(chainConfig)
signer, err := xcFactory.NewSigner(chainConfig, privateKey)
```

### Supported Chains

See `chain/` directory for all implementations. Major chains include:

- **Bitcoin family**: bitcoin, bitcoin_cash, dogecoin (UTXO-based)
- **EVM chains**: evm, evm_legacy (Ethereum and compatible chains)
- **Cosmos ecosystem**: cosmos (+ derivatives like Terra, Injective, XPLA)
- **Modern chains**: solana, sui, aptos, ton, tron
- **Substrate-based**: substrate (Polkadot, Kusama, Astar, etc.)
- **Specialized**: hyperliquid, kaspa, xlm (Stellar), xrp

Each chain configuration is defined in `factory/defaults/chains/mainnet.yaml` and `testnet.yaml`.

### Cross-Chain Types

Core types are defined in the root package (`github.com/cordialsys/crosschain`):

- `xc.Tx` - Universal transaction interface
- `xc.TxInput` - Chain-specific inputs needed for transaction construction
- `xc.TxInfo` - Normalized transaction information across chains
- `xc.AmountBlockchain` - Blockchain-native amounts (big integers)
- `xc.AmountHumanReadable` - Human-readable decimal amounts
- `xc.Address`, `xc.TxHash`, `xc.ContractAddress` - Type-safe string wrappers

## Important Conventions

### Chain Configuration

- Chain configurations use YAML in `factory/defaults/chains/`
- Each chain has a unique `Driver` enum value (e.g., `DriverBitcoin`, `DriverEVM`)
- Chain-specific settings: RPC URLs, chain IDs, decimals, gas multipliers, staking configs
- The `network` field determines mainnet vs testnet behavior

### Address Handling

- Use `address.AddressOption` for chain-specific address format selection (e.g., Bitcoin legacy vs segwit vs taproot)
- Address builders normalize and validate addresses per chain rules
- Many chains use HD path derivation - see `chain_coin_hd_path` in chain configs

### Amount Conversion

Always convert between human-readable and blockchain amounts using the factory:

```go
blockchain, err := factory.ConvertAmountStrToBlockchain(cfg, "1.5")
human, err := factory.ConvertAmountToHuman(cfg, blockchainAmount)
```

### Transaction Flow

Standard transaction flow:

1. Get inputs: `client.FetchTransferInput(ctx, args)`
2. Build tx: `builder.Transfer(args, input)`
3. Sign tx: `tx.Sign(signatures)` or `signer.Sign(req)`
4. Submit: `client.SubmitTx(ctx, tx)`

### Staking

Staking support varies by chain. Check `chain/<name>/client.go` for `StakingClient` implementation.

- Substrate chains use `SubtensorModule.add_stake`/`remove_stake`
- Cosmos chains use native staking modules
- Ethereum uses third-party staking providers (Kiln, Twinstake)

### Testing Notes

- Integration tests are tagged with `not_ci` and excluded from `make test`
- CI tests in `ci/` directory test against real networks (use sparingly)
- Use `testutil/` helpers for test setup
- Most chains have devnet Docker images for local testing

## Current Branch Context

Branch: `substrate-staking`
Modified: `chain/substrate/tx_input/calls.go` (working on staking calls)
Chain configs: `factory/defaults/chains/testnet.yaml`

## Debugging Tips

- Use `-v` flag with `xc` CLI for verbose logging
- Set `logrus.SetLevel(logrus.DebugLevel)` for detailed logs
- Check `client/errors/` for chain-specific error handling
- RPC errors are normalized through `factory.CheckError()`
