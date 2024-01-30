require("@nomicfoundation/hardhat-toolbox");

var chainId = process.env.CHAIN_ID
chainId = chainId || "2"

console.log("CHAIN_ID:", chainId)

/** @type import('hardhat/config').HardhatUserConfig */
module.exports = {
  solidity: "0.8.17",
  networks: {
    hardhat: {
      chainId: parseInt(chainId),
      hardfork: 'shanghai',
      baseFeePerGas: "0",
      mining: {
        auto: false,
        interval: 3000
      }
    },
  }
};