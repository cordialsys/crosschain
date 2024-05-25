require("@nomicfoundation/hardhat-toolbox");

var chainId = process.env.CHAIN_ID
chainId = chainId || "2"

let port = process.env.RPC_PORT;
if (!port) {
  port = "10000";
}
console.log("CHAIN_ID:", chainId, "PORT:", port)

/** @type import('hardhat/config').HardhatUserConfig */
module.exports = {
  solidity: "0.8.17",
  networks: {
    localhost: {
      url: `http://127.0.0.1:${port}`,
    },
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