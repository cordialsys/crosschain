#!/bin/bash

npm list | grep hardhat

echo "hardhat version:"
npx hardhat --version

npx hardhat node --hostname 0.0.0.0 --port ${RPC_PORT} &
# deploy some tokens

# First run deploys contract with address 0x5FbDB2315678afecb367f032d93F642f64180aa3
npx hardhat run scripts/deploy.js --network localhost

# Second run: 0xe7f1725E7734CE288F8367e1Bb143E90bb3F0512
# npx hardhat run scripts/deploy.js --network localhost

# Third run: 0x9fE46736679d2D9a65F0992F2272dE9f3c7fa6e0
# npx hardhat run scripts/deploy.js --network localhost
wait