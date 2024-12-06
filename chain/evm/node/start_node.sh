#!/bin/bash

npm list | grep hardhat

echo "hardhat version:"
npx hardhat --version

npx hardhat node --hostname 0.0.0.0 --port ${RPC_PORT} &
# deploy some tokens
# npx hardhat run scripts/deploy.js --network localhost
# npx hardhat run scripts/deploy.js --network localhost
# npx hardhat run scripts/deploy.js --network localhost
wait