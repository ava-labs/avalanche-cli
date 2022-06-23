// We require the Hardhat Runtime Environment explicitly here. This is optional
// but useful for running the script in a standalone fashion through `node <script>`.
//
// When running the script with `npx hardhat run <script>` you'll find the Hardhat
// Runtime Environment's members available in the global scope.
import { ethers } from "hardhat"

import { Greeter } from "../typechain/Greeter"
import { abi } from "../artifacts/contracts/Greeter.sol/Greeter.json"
import { Greeter as GreeterAddress } from "../greeter.json"
import { assert } from "console"

async function main() {
  // We get the deployed contract
  let [signer1] = await ethers.getSigners()
  const greeter = new ethers.Contract(GreeterAddress, abi, signer1) as Greeter

  console.log("Greeter found at:", greeter.address)

  const expectedGreeting = "Hello, subnet!"
  let currentGreeting = await greeter.greet()
  console.log("Current greeting:", currentGreeting)

  if (currentGreeting != expectedGreeting) {
    throw new Error(`greeter contract not updated, expected ${currentGreeting} to be ${expectedGreeting}`)
  }
}

// We recommend this pattern to be able to use async/await everywhere
// and properly handle errors.
main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
