import Zemu, { DEFAULT_START_OPTIONS } from "@zondax/zemu";

const Resolve = require('path').resolve

const appPath = Resolve('app_s.elf')
const waitTimeout = 60000;
const waitUntilClose = 1000;
const grpcPort = 3002;
const transportPort = 9998;
const speculosApiPort = 5000;

const numApprovals = parseInt(process.argv[2], 10);
const waitSeconds = parseInt(process.argv[3], 10);
var appSeed = "equip will roof matter pink blind book anxiety banner elbow sun young";
if (process.argv[4] != undefined) {
  appSeed = process.argv[4];
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

  console.log("")
  console.log("SIMULATED LEDGER DEV READY")

  await Zemu.sleep(waitSeconds*1000);

  for (let i = 0; i < numApprovals; i++) {
    await sim.deleteEvents();
    await sim.waitUntilScreenIs(readyScreen, waitTimeout);
    await sim.waitUntilScreenIsNot(readyScreen, waitTimeout);
    await sim.navigateUntilText(".", "pp", "APPROVE", false, false, 0, waitTimeout, true);
  }

  await Zemu.sleep(waitUntilClose);
  await sim.close();
  await Zemu.stopAllEmuContainers();
}

main()
  .then(() => process.exit(0))
  .catch(error => {
    console.error(error)
    process.exit(1)
  })
