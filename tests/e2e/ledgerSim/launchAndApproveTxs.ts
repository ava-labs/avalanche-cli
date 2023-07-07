import Zemu, { DEFAULT_START_OPTIONS } from "@zondax/zemu";

const Resolve = require('path').resolve

/*
const catchExit = async () => {
  process.on('SIGINT', () => {
    Zemu.stopAllEmuContainers()
  })
}
*/

export const APP_SEED = 'equip will roof matter pink blind book anxiety banner elbow sun young'

export const defaultOptions = {
  ...DEFAULT_START_OPTIONS,
 // logging: true,
  custom: `-s "${APP_SEED}"`,
  startText: 'Ready'
}

const appPath = Resolve('app_s.elf')
const ledgerModel = 'nanos'
const numApprovals = parseInt(process.argv[2], 10);

async function main() {
  console.log("Zemu demo")

  //await catchExit();
  const sim = new Zemu(appPath, {}, "127.0.0.1", 9998, 5000);
  //await Zemu.checkAndPullImage();
  //await Zemu.stopAllEmuContainers();
  //await sim.start({ ...defaultOptions, model: "nanos" });
  //
  //await sim.start({ ...DEFAULT_START_OPTIONS, model: "nanos" });
  await sim.start(DEFAULT_START_OPTIONS)
  
  sim.startGRPCServer("localhost", 3002);

  // wait until avalanche app ready screen and take screen snapshot
  await sim.waitForText("Avalanche", 60000, true);
  await sim.waitForText("Ready", 60000, true);
  const readyScreen = await sim.snapshot();

  console.log("READY")

  for (let i = 0; i < numApprovals; i++) {
    await sim.deleteEvents();
    await sim.waitUntilScreenIs(readyScreen, 60000);
    await sim.waitUntilScreenIsNot(readyScreen, 60000);
    await sim.navigateUntilText(".", "pp", "APPROVE", false, false, 0, 60000, true);
  }

  await new Promise(r => setTimeout(r, 1000));

  //await sim.close();
  //await Zemu.stopAllEmuContainers();
}

main()
  .then(() => process.exit(0))
  .catch(error => {
    console.error(error)
    process.exit(1)
  })
