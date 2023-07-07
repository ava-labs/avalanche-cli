import Zemu, { DEFAULT_START_OPTIONS } from "@zondax/zemu";

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

async function main() {
  console.log("Zemu demo")
  //await catchExit();
  const sim = new Zemu("/home/fm/Workdir/projects/ledger-avalanche/build/output/app_s.elf", {}, "127.0.0.1", 9998, 5000);
  //await Zemu.checkAndPullImage();
  //await Zemu.stopAllEmuContainers();
  await sim.start({ ...defaultOptions, model: "nanos" });
  //await sim.start({ ...DEFAULT_START_OPTIONS, model: "nanos" });
  sim.startGRPCServer("localhost", 3002);

  // wait until avalanche app ready screen and take screen snapshot
  await sim.waitForText("Avalanche", 60000, true);
  await sim.waitForText("Ready", 60000, true);
  const readyScreen = await sim.snapshot();
  console.log("READY")

  // approve import tx
  await sim.deleteEvents();
  await sim.waitUntilScreenIs(readyScreen, 60000);
  await sim.waitUntilScreenIsNot(readyScreen, 60000);
  await sim.navigateUntilText(".", "pp", "APPROVE", false, false, 0, 60000, true);

  // approve create subnet tx
  await sim.deleteEvents();
  await sim.waitUntilScreenIs(readyScreen, 60000);
  await sim.waitUntilScreenIsNot(readyScreen, 60000);
  await sim.navigateUntilText(".", "pp", "APPROVE", false, false, 0, 60000, true);
  
  // approve create chain tx
  await sim.deleteEvents();
  await sim.waitUntilScreenIs(readyScreen, 60000);
  await sim.waitUntilScreenIsNot(readyScreen, 60000);
  await sim.navigateUntilText(".", "pp", "APPROVE", false, false, 0, 60000, true);
 
  // approve add validator 1
  await sim.deleteEvents();
  await sim.waitUntilScreenIs(readyScreen, 60000);
  await sim.waitUntilScreenIsNot(readyScreen, 60000);
  await sim.navigateUntilText(".", "pp", "APPROVE", false, false, 0, 60000, true);

  // approve add validator 2
  await sim.deleteEvents();
  await sim.waitUntilScreenIs(readyScreen, 60000);
  await sim.waitUntilScreenIsNot(readyScreen, 60000);
  await sim.navigateUntilText(".", "pp", "APPROVE", false, false, 0, 60000, true);

  // approve add validator 3
  await sim.deleteEvents();
  await sim.waitUntilScreenIs(readyScreen, 60000);
  await sim.waitUntilScreenIsNot(readyScreen, 60000);
  await sim.navigateUntilText(".", "pp", "APPROVE", false, false, 0, 60000, true);

  // approve add validator 4
  await sim.deleteEvents();
  await sim.waitUntilScreenIs(readyScreen, 60000);
  await sim.waitUntilScreenIsNot(readyScreen, 60000);
  await sim.navigateUntilText(".", "pp", "APPROVE", false, false, 0, 60000, true);

  // approve add validator 5
  await sim.deleteEvents();
  await sim.waitUntilScreenIs(readyScreen, 60000);
  await sim.waitUntilScreenIsNot(readyScreen, 60000);
  await sim.navigateUntilText(".", "pp", "APPROVE", false, false, 0, 60000, true);

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
