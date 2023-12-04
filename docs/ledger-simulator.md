# How this works

## How the simulator is started and configured

The script `launchAndApproveTxs.ts` executes the ledger simulator.

It uses the `@zondax/zemu` js library to:

- Download the docker image for the simulator (if needed)
- Set the ledger seed to the default or the user-given one
- Execute the docker container for the simulator by passing to it the avalanche app binary `app_s.elf` (ledger nano s device). That starts the simulated avalanche app.
- Create a rpc entry point to the simulated avalanche app so as the golang client ledger library can connect to the simulator (instead of a real device)
- Previous steps can take some time. Once the app and rpc entry is ready, it prints a custom msg `SIMULATED LEDGER DEV READY` as a means to communicate 
  with the test code (or the user) that the simulator can start receiving requests (eg connect to it, ask for addresses, etc).
- Instructs the simulated app (using simulated button presses from the zemu library) to sign `numApprovals` transactions. This number of transactions
  to be approved must be in concordance with the number of transactions that the golang test asks the ledger to approve. 
- Wait for all transactions to be received and approved. In the meantime, the avalanche app can be queried by golang code so as
  for example to get the ledger addresses.
- Wait for the user to press enter to end the simulator. In the meantime, the avalanche app can be queried by golang code.
- Close the rpc entry point, closes the simulator, stops and remove the docker container.

So, two main points of interaction with the simulated avalanche ledger app are available:

1. Interaction with the ledger from the golang app as if it were a real device: asking for addresses, sending txs to be signed.
2. Interaction with the simulated physical ledger from the typescript code: pressing the ledger buttons in order to sign the transactions.

The script receives two arguments: 

- the number of transactions to sign (could be zero)
- the seed for the ledger, that controls the ledger addresses (could be one word)

Eg to start the ledger app and sign one transaction (will wait -with timeout- for that transaction to be sent):

```bash
ts-node launcheAndApproveTxs.ts 1 ledger-seed
```

It can be executed directly by command line, or from inside golang test code.

Note: Besides the typescript based interaction with the ledger's buttons and screen, a web based interaction is also
provided by the simulator, and available under `http://localhost:5000`

## How a golang test connects to the simulator instead of a physical device

As it is currently implemented, a tag must be passed to the build step for avalanche-cli and e2e test binary:

```
-tags ledger_zemu
```

With these tag, the ledger golang library will try to connect to an rpc endpoint instead of a usb device 
when asked to send requests to ledger (query for addresses, etc).

Currently the rpc endpoint is hardcoded in ledger golang library to: `127.0.0.1:3002`

## How a golang test executes and interacts with the typescript script

The test should call `utils.StartLedgerSim(numApprovals, ledgerSeed, showStdout)` by providing:

- number of txs to be approved by the ledger
- seed for the ledger
- wether to show stdout

The function returns two channels:

- first one should be closed to notify the simulator that it can start shutdown process
- second one should be waited for to be confident that the simulator stopped.

See example on `deploy subnet to mainnet` test

It is expected for a test to check env var `LEDGER_SIM`, if it is set to `true`, launch and wait for
the simulator, if not, the test is expected to operate agains a real device.

## Ledger device status for avalanche-cli tests interaction

Latest avalanche ledger app downloadable version `v0.7.2` (and also latest ledger live official version `v0.7.0`) can not interact with tests
as it does not support avalanche-cli local network id 1337.

For that, currently the tests operate against a modified version of `v0.7.2`, available as `app_s.elf` binary.

For a real ledger device to be used with the tests, it should be loaded with a supporting version, currently available on dev branch of ledger-avalanche.

It is expected for next downloadable version to:

- support network id 1337
- provide elf binary downloads

With that elements provided, CLI e2e could start downloading latest ledger app and using it on CI.

## How to execute the test script

By default, the e2e script `scripts/run.e2e.sh` will try to execute all tests (including ledger based ones),
and use physical ledger device.

In order to used a simulated ledger device with the script, the env var LEDGER_SIM should be set to `true`.

For example:

```bash
LEDGER_SIM=true scripts/run.e2e.sh
```

Will execute all e2e tests using a simulated ledger device.

In order to execute a specific ledger test, it must be provided in filter arg.

For example:

```bash
LEDGER_SIM=true scripts/run.e2e.sh --filter 'deploy subnet to mainnet'
```

Another example that always needs to use ledger sim due to test implementation:

```bash
LEDGER_SIM=true scripts/run.e2e.sh --filter multisig
```

