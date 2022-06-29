// We require the Hardhat Runtime Environment explicitly here. This is optional
// but useful for running the script in a standalone fashion through `node <script>`.
//
// When running the script with `npx hardhat run <script>` you'll find the Hardhat
// Runtime Environment's members available in the global scope.
import { ethers } from "hardhat"

import { Greeter } from "../typechain/Greeter"

async function main() {
  // Hardhat always runs the compile task when running scripts with its command
  // line interface.
  //
  // If this script is run directly using `node` you may want to call compile
  // manually to make sure everything is compiled
  // await hre.run('compile');

  // We get the contract to deploy
  const GreeterFactory = await ethers.getContractFactory("Greeter")
  const greeter = await GreeterFactory.deploy("Hello, Hardhat!") as Greeter

  await greeter.deployed()

  console.log("Greeter deployed to:", greeter.address)

  console.log("Current greeting:", await greeter.greet())
  console.log("Updating greeting")

  await greeter.setGreeting("Hello, subnet!")
  console.log("Updated greeting:", await greeter.greet())
}

// We recommend this pattern to be able to use async/await everywhere
// and properly handle errors.
main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
