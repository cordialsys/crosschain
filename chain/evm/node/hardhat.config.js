require("@nomicfoundation/hardhat-toolbox");

/** @type import('hardhat/config').HardhatUserConfig */
module.exports = {
  solidity: "0.8.17",
  networks: {
    hardhat: {
      chainId: 1337,
      hardfork: 'london',
      baseFeePerGas: "10000000000",
    },
  }
};