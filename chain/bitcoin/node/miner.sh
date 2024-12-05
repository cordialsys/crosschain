#!/bin/bash

# This just force mines a new block every 5s.

# create an initial wallet, retrying until it succeeds when the native RPC node is up.
false
while [  ! "$?" = "0" ] ; do
    sleep .2
    bitcoin-cli -rpcuser=$USERNAME -rpcpassword=$PASSWORD -regtest createwallet 'faucet'
done

# mine a 100 blocks to a derived address on the wallet
# we need at least 100 confirmations before block reward is sent.
bitcoin-cli -regtest -rpcuser=$USERNAME -rpcpassword=$PASSWORD generatetoaddress 101 $(bitcoin-cli -rpcuser=$USERNAME -rpcpassword=$PASSWORD -regtest getnewaddress)

while [ 1 ] ; do
    
    # mine another block
    bitcoin-cli -regtest -rpcuser=$USERNAME -rpcpassword=$PASSWORD generatetoaddress 1 $(bitcoin-cli -rpcuser=$USERNAME -rpcpassword=$PASSWORD -regtest getnewaddress)

    # our "block time"
    sleep 5

done