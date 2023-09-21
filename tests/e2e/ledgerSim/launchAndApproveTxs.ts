import { createInterface } from "readline";
import Zemu, { DEFAULT_START_OPTIONS } from "@zondax/zemu";

const rl = createInterface({
	input: process.stdin,
	output: process.stdout
});

const Resolve = require('path').resolve

const appPath = Resolve('app_s.elf')
const waitTimeout = 60000;
const waitUntilClose = 1000;
const grpcPort = 3002;
const transportPort = 9998;
const speculosApiPort = 5000;

var numApprovals = parseInt(process.argv[2], 10);
if (isNaN(numApprovals)) {
	numApprovals = 0;
}

var appSeed = "equip will roof matter pink blind book anxiety banner elbow sun young";
if (process.argv[3] != undefined) {
  appSeed = process.argv[3];
}

const options = {
  ...DEFAULT_START_OPTIONS,
  custom: `-s "${appSeed}"`,
}

async function main() {
  const sim = new Zemu(appPath, {}, "127.0.0.1", transportPort, speculosApiPort);

  await Zemu.checkAndPullImage();
  await Zemu.stopAllEmuContainers();

  await sim.start(options)
  
  sim.startGRPCServer("localhost", grpcPort);

  await sim.waitForText("Avalanche", waitTimeout, true);
  await sim.waitForText("Ready", waitTimeout, true);
  const readyScreen = await sim.snapshot();

  console.log("");
  console.log("SIMULATED LEDGER DEV READY");

  for (let i = 0; i < numApprovals; i++) {
    console.log("Ready to approve tx", i+1, "of", numApprovals);
    await sim.deleteEvents();
    await sim.waitUntilScreenIs(readyScreen, waitTimeout);
    await sim.waitUntilScreenIsNot(readyScreen, waitTimeout);
    await sim.navigateUntilText(".", "pp", "APPROVE", false, false, 0, waitTimeout, true);
  }

  await Zemu.sleep(waitUntilClose);

  await new Promise(resolve => {
    rl.question("PRESS ENTER TO END SIMULATOR\n", resolve)
  });

  await sim.close();
  await Zemu.stopAllEmuContainers();
}

main()
  .then(() => process.exit(0))
  .catch(error => {
    console.error(error)
    process.exit(1)
  })
