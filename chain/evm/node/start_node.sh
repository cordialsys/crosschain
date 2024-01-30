#!/bin/bash

npm list | grep hardhat

echo "hardhat version:"
npx hardhat --version

npx hardhat node $@ &
# deploy some tokens
npx hardhat run scripts/deploy.js --network localhost
npx hardhat run scripts/deploy.js --network localhost
npx hardhat run scripts/deploy.js --network localhost
wait