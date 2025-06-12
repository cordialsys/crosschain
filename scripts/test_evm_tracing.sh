#!/bin/bash
set -xe

RPC=$1

if [ -z "$RPC" ]; then
    echo "Usage: $0 <rpc_url>"
    echo "Must provide an mainnet ETH RPC URL that supports both trace_transaction and debug_traceTransaction"
    exit 1
fi

# Mainnet ETH transactions transaction vectors
HASHES=(
    "0x452ad9db5bf596fc44b3a75edaa03871bbb117d747415579b91b12c5037a59e6"
    "0x66c1e367e9ad0c75e6bb9f25d0b8153ea03f9f46f4f2bce8e72b54c5cdd9dcc5" 
    "0x9f11faf200062d5abf088867728aa732d8025fd0f5e67bec808fe2e69a1a5673"
    "0x6d5cbd491d13864357e8eb8d7df30a1f01e2d366d9985bc8b97ffc11f15f4284"
    "0xbacc59f21eea37b3edadf105e90a915899bcc58d3d62e9a42f515527aa69397e"
    "0x9ab1a186629d18a0798165f5bf0ef5b59bcfb98fd6de633943484d593cc9a2f6"
    "0x670a9678307aafcb5b6c3be51f9254b87b0ef82e87cc2d075c1addd787aa5d9a"
    "0xf386fb30d9affc27baa51d3e055af400de76b444438437e3ff6cc90d650e4498"
    "0x0b37d1416d10581e0ae1880b1a7e4b3da98660e594db0575376d9a0a06b2c6c5"
)

for hash in "${HASHES[@]}"; do
    echo "Testing hash: $hash"
    EVM_DEBUG_TRACE=1 xc --chain ETH tx-info -vv "$hash" | grep -v confirmations > "tx_debug.json"
    EVM_TRACE=1 xc --chain ETH tx-info -vv "$hash" | grep -v confirmations > "tx_trace.json"
    diff "tx_debug.json" "tx_trace.json" 
    rm "tx_debug.json" "tx_trace.json"
done


