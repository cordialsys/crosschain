#!/bin/bash


anvil --version

anvil --hardfork prague --host 0.0.0.0 --port ${RPC_PORT} --block-time 2 &

while ! nc -z localhost ${RPC_PORT}; do sleep .25; echo waiting for port to open; done

# This should match the basic_smart_account address used by evm tx package.
# expected address: 0x91A4a87AB11aE1cbB2c8ba981AEa32aeCF54Dfc0
# See also ci/untils.go
export EVM_SALT=0xbdfee0231e0903cde9ca6fd75d08a500062dc3d87718f712bc6958ed697617c3
forge script ./script/DeployBasicSmartAccount.s.sol --rpc-url http://localhost:${RPC_PORT} --broadcast --private-key ${PRIVATE_KEY}

# deploy some tokens:
# expected address: 0xe1D98dC57ea81d94Bb15b9a8ca5c0075C3d7a0C1
export EVM_SALT=0x0000000000000000000000000000000000000000000000000000000000000002
forge script ./script/DeployToken.s.sol --rpc-url http://localhost:${RPC_PORT} --broadcast --private-key ${PRIVATE_KEY}

wait