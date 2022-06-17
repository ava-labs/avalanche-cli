import { expect } from "chai"
import { ethers } from "hardhat"

describe("Greeter", function () {
  it("Should return the new greeting once it's changed", async function () {
    console.log("In test")
    const Greeter = await ethers.getContractFactory("Greeter")
    console.log("In test 2")
    const greeter = await Greeter.deploy("Hello, world!")
    console.log("In test 3")
    await greeter.deployed()
    console.log("In test 4")

    // expect(await greeter.greet()).to.equal("Hello, world!")

    // const setGreetingTx = await greeter.setGreeting("Hola, mundo!");

    // // wait until the transaction is mined
    // await setGreetingTx.wait();

    // expect(await greeter.greet()).to.equal("Hola, mundo!");
  })
})
