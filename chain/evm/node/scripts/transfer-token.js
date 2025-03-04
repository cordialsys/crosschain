

var contractAddress = process.env.CONTRACT;
var amountToTransfer = process.env.AMOUNT;
var to = process.env.TO;

async function main() {
    // ethers is available in the global scope
    const [deployer] = await ethers.getSigners();
    console.log(
      `Sending ${amountToTransfer} token from:`,
      await deployer.getAddress(),
      "to:",
      to
    );
  
    console.log("Account balance:", (await deployer.getBalance()).toString());
  
    if ((await ethers.provider.getCode(contractAddress)) === "0x") {
        console.error("Contract not deployed");
        process.exit(1);
      }
    const token = await ethers.getContractAt("Token", contractAddress);

    const tx = await token.transfer(to, parseInt(amountToTransfer));
    await tx.wait();
  
    console.log(`Transferred ${amountToTransfer} token to ${to}`);
}
main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });